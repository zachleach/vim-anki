package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type ScheduleRow struct {
	QuestionHash    string
	FilePath        string
	DueDate         string
	ReviewDateIndex int
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
		CREATE TABLE IF NOT EXISTS tracked_file (
			file_path TEXT PRIMARY KEY,
			created_at TEXT
		);
		CREATE TABLE IF NOT EXISTS schedule_info (
			question_hash TEXT PRIMARY KEY,
			file_path TEXT,
			due_date TEXT NOT NULL,
			review_date_index INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS review_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			question_hash TEXT,
			reviewed_at TEXT,
			outcome TEXT,
			review_date_index INTEGER
		);
	`)
	return err
}

func getScheduleInfo(db *sql.DB, hash string) (*ScheduleRow, error) {
	row := db.QueryRow("SELECT question_hash, file_path, due_date, review_date_index FROM schedule_info WHERE question_hash = ?", hash)
	var r ScheduleRow
	err := row.Scan(&r.QuestionHash, &r.FilePath, &r.DueDate, &r.ReviewDateIndex)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func isDue(db *sql.DB, hash string) bool {
	info, err := getScheduleInfo(db, hash)
	if err != nil || info == nil {
		return true // new cards are always due
	}
	dueDate, err := time.Parse("2006-01-02", info.DueDate)
	if err != nil {
		return true
	}
	today := time.Now().Truncate(24 * time.Hour)
	return !dueDate.After(today)
}

func updateSchedule(db *sql.DB, hash, filePath string, result int) {
	info, _ := getScheduleInfo(db, hash)
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
		"INSERT OR REPLACE INTO schedule_info (question_hash, file_path, due_date, review_date_index) VALUES (?, ?, ?, ?)",
		hash, filePath, dueDate, newIndex,
	)

	// log the review
	if result == Wrong || result == Correct {
		outcome := "wrong"
		if result == Correct {
			outcome = "correct"
		}
		insertReviewLog(db, hash, outcome, newIndex)
	}
}

func insertReviewLog(db *sql.DB, hash, outcome string, index int) {
	db.Exec(
		"INSERT INTO review_log (question_hash, reviewed_at, outcome, review_date_index) VALUES (?, datetime('now'), ?, ?)",
		hash, outcome, index,
	)
}

func countDueInFile(db *sql.DB, filePath string) int {
	chunks, err := parseChunks(filePath)
	if err != nil {
		return 0
	}
	count := 0
	for _, c := range chunks {
		if isDue(db, computeHash(c.QuestionLine)) {
			count++
		}
	}
	return count
}

func trackFile(db *sql.DB, filePath string) {
	abs, _ := filepath.Abs(filePath)
	db.Exec(
		"INSERT OR IGNORE INTO tracked_file (file_path, created_at) VALUES (?, datetime('now'))",
		abs,
	)
}

func getTrackedFiles(db *sql.DB) []string {
	rows, err := db.Query("SELECT file_path FROM tracked_file ORDER BY file_path")
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

	trackFile(db, abs)

	chunks, err := parseChunks(abs)
	if err != nil {
		return
	}

	// collect current hashes
	currentHashes := make(map[string]bool)
	today := time.Now().Format("2006-01-02")

	for _, c := range chunks {
		hash := computeHash(c.QuestionLine)
		currentHashes[hash] = true

		// insert new questions only (don't overwrite existing schedule)
		info, _ := getScheduleInfo(db, hash)
		if info == nil {
			db.Exec(
				"INSERT INTO schedule_info (question_hash, file_path, due_date, review_date_index) VALUES (?, ?, ?, ?)",
				hash, abs, today, 1,
			)
		}
	}

	// remove orphaned entries for this file
	rows, _ := db.Query("SELECT question_hash FROM schedule_info WHERE file_path = ?", abs)
	if rows != nil {
		defer rows.Close()
		var orphans []string
		for rows.Next() {
			var h string
			rows.Scan(&h)
			if !currentHashes[h] {
				orphans = append(orphans, h)
			}
		}
		for _, h := range orphans {
			db.Exec("DELETE FROM schedule_info WHERE question_hash = ?", h)
		}
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

func restoreScheduleState(db *sql.DB, hash string, prev *ScheduleRow) {
	if prev == nil {
		db.Exec("DELETE FROM schedule_info WHERE question_hash = ?", hash)
	} else {
		db.Exec(
			"INSERT OR REPLACE INTO schedule_info (question_hash, file_path, due_date, review_date_index) VALUES (?, ?, ?, ?)",
			prev.QuestionHash, prev.FilePath, prev.DueDate, prev.ReviewDateIndex,
		)
	}
}
