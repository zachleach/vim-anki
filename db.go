package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type ScheduleRow struct {
	Question        string
	FilePath        string
	DueDate         string
	ReviewDateIndex int
	Flagged         bool
}

func dbPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".notes.db")
}

func openDB() (*sql.DB, error) {
	return sql.Open("sqlite3", dbPath())
}

func initDB(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schedule_info (
			question TEXT PRIMARY KEY,
			file_path TEXT,
			due_date TEXT NOT NULL,
			review_date_index INTEGER NOT NULL,
			flagged INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS review_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			question TEXT,
			reviewed_at TEXT,
			outcome TEXT,
			review_date_index INTEGER
		);
	`)
	if err != nil {
		return err
	}

	// migrate: add flagged column if missing
	db.Exec("ALTER TABLE schedule_info ADD COLUMN flagged INTEGER NOT NULL DEFAULT 0")

	// migrate: move data from flagged_questions table if it exists
	db.Exec("UPDATE schedule_info SET flagged = 1 WHERE question IN (SELECT question FROM flagged_questions)")
	db.Exec("DROP TABLE IF EXISTS flagged_questions")

	return nil
}

func getScheduleInfo(db *sql.DB, question string) (*ScheduleRow, error) {
	row := db.QueryRow("SELECT question, file_path, due_date, review_date_index, flagged FROM schedule_info WHERE question = ?", question)
	var r ScheduleRow
	err := row.Scan(&r.Question, &r.FilePath, &r.DueDate, &r.ReviewDateIndex, &r.Flagged)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func flagQuestionDB(db *sql.DB, question, filePath string) {
	db.Exec("UPDATE schedule_info SET flagged = 1 WHERE question = ?", question)
}

func unflagQuestion(db *sql.DB, question string) {
	result, _ := db.Exec("UPDATE schedule_info SET flagged = 0 WHERE question = ? AND flagged = 1", question)
	if n, _ := result.RowsAffected(); n > 0 {
		fmt.Println("Unflagged question.")
	} else {
		fmt.Println("Question not found in flagged list.")
	}
}

func listFlagged(db *sql.DB) {
	rows, err := db.Query("SELECT question, file_path FROM schedule_info WHERE flagged = 1 ORDER BY file_path")
	if err != nil {
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var question, filePath string
		rows.Scan(&question, &filePath)

		displayPath := filePath
		if home, err := os.UserHomeDir(); err == nil {
			if rel, _ := filepath.Rel(home, filePath); !strings.HasPrefix(rel, "..") {
				displayPath = "~/" + rel
			}
		}

		q := question
		if len(q) > 2 {
			q = q[2:] // strip >\t prefix
		}
		if len(q) > 60 {
			q = q[:60] + "..."
		}

		fmt.Printf("%s\n  %s\n  %s\n\n", displayPath, q, question)
		count++
	}

	if count == 0 {
		fmt.Println("No flagged questions.")
	} else {
		fmt.Printf("%d flagged question(s).\n", count)
	}
}

func isDue(db *sql.DB, question string) bool {
	info, err := getScheduleInfo(db, question)
	if err != nil || info == nil {
		return true // new cards are always due
	}
	if info.Flagged {
		return false
	}
	dueDate, err := time.Parse("2006-01-02", info.DueDate)
	if err != nil {
		return true
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return !dueDate.After(today)
}

func updateSchedule(db *sql.DB, question, filePath string, result int) {
	info, _ := getScheduleInfo(db, question)
	currentIndex := 1 // new cards start at index 1
	if info != nil {
		currentIndex = info.ReviewDateIndex
	}

	today := time.Now().Format("2006-01-02")
	var newIndex int
	var dueDate string

	switch result {
	case Wrong:
		newIndex = 0
		dueDate = today
	case Correct:
		newIndex = currentIndex + 1
		if newIndex >= len(ScheduleIntervals) {
			newIndex = len(ScheduleIntervals) - 1
		}
		due := time.Now().AddDate(0, 0, ScheduleIntervals[newIndex])
		dueDate = due.Format("2006-01-02")
	case Skip:
		newIndex = currentIndex
		dueDate = today
	}

	db.Exec(
		"INSERT OR REPLACE INTO schedule_info (question, file_path, due_date, review_date_index) VALUES (?, ?, ?, ?)",
		question, filePath, dueDate, newIndex,
	)

	// log the review
	if result == Wrong || result == Correct {
		outcome := "wrong"
		if result == Correct {
			outcome = "correct"
		}
		insertReviewLog(db, question, outcome, newIndex)
	}
}

func insertReviewLog(db *sql.DB, question, outcome string, index int) {
	db.Exec(
		"INSERT INTO review_log (question, reviewed_at, outcome, review_date_index) VALUES (?, datetime('now'), ?, ?)",
		question, outcome, index,
	)
}

func countDueInFile(db *sql.DB, filePath string) int {
	chunks, err := parseChunks(filePath)
	if err != nil {
		return 0
	}
	count := 0
	for _, c := range chunks {
		if isDue(db, c.QuestionLine) {
			count++
		}
	}
	return count
}

func getTrackedFiles(db *sql.DB) []string {
	rows, err := db.Query("SELECT DISTINCT file_path FROM schedule_info ORDER BY file_path")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var files []string
	for rows.Next() {
		var f string
		rows.Scan(&f)
		files = append(files, f)
	}
	return files
}

func syncFileQuestions(db *sql.DB, filePath string) {
	abs, _ := filepath.Abs(filePath)

	if !hasQuestionLines(abs) {
		return
	}

	chunks, err := parseChunks(abs)
	if err != nil {
		return
	}

	// collect current questions
	currentQuestions := make(map[string]bool)
	today := time.Now().Format("2006-01-02")

	for _, c := range chunks {
		currentQuestions[c.QuestionLine] = true

		// insert new questions only (don't overwrite existing schedule)
		info, _ := getScheduleInfo(db, c.QuestionLine)
		if info == nil {
			db.Exec(
				"INSERT INTO schedule_info (question, file_path, due_date, review_date_index) VALUES (?, ?, ?, ?)",
				c.QuestionLine, abs, today, 1,
			)
		} else if info.FilePath != abs {
			db.Exec("UPDATE schedule_info SET file_path = ? WHERE question = ?", abs, c.QuestionLine)
		}
	}
}

func syncAllTrackedFiles(db *sql.DB) {
	for _, f := range getTrackedFiles(db) {
		if _, err := os.Stat(f); err != nil {
			continue // file gone — skip, questions may reappear in another file
		}
		syncFileQuestions(db, f)
	}
}

func forgetFileSchedule(db *sql.DB, filePath string) {
	abs, _ := filepath.Abs(filePath)
	result, _ := db.Exec("DELETE FROM schedule_info WHERE file_path = ?", abs)
	if n, _ := result.RowsAffected(); n > 0 {
		fmt.Printf("Forgot %d question(s) from schedule.\n", n)
	} else {
		fmt.Println("No scheduled questions found for this file.")
	}
}

func restoreScheduleState(db *sql.DB, question string, prev *ScheduleRow) {
	if prev == nil {
		db.Exec("DELETE FROM schedule_info WHERE question = ?", question)
	} else {
		db.Exec(
			"INSERT OR REPLACE INTO schedule_info (question, file_path, due_date, review_date_index) VALUES (?, ?, ?, ?)",
			prev.Question, prev.FilePath, prev.DueDate, prev.ReviewDateIndex,
		)
	}
}
