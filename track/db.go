package main

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type food struct {
	name     string
	calories int
}

func trackDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".personal.db")
}

func openDB() (*sql.DB, error) {
	return sql.Open("sqlite3", trackDBPath())
}

func initDB(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS foods (
			name     TEXT PRIMARY KEY,
			calories INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS log (
			id       INTEGER PRIMARY KEY AUTOINCREMENT,
			date     TEXT NOT NULL,
			name     TEXT NOT NULL,
			quantity REAL NOT NULL,
			calories REAL NOT NULL
		);
		CREATE TABLE IF NOT EXISTS weight (
			date TEXT PRIMARY KEY,
			lbs  REAL NOT NULL
		);
	`)
	return err
}

func upsertFood(db *sql.DB, name string, calories int) error {
	_, err := db.Exec("INSERT OR REPLACE INTO foods (name, calories) VALUES (?, ?)", name, calories)
	return err
}

func logEntry(db *sql.DB, date, name string, quantity, calories float64) error {
	_, err := db.Exec("INSERT INTO log (date, name, quantity, calories) VALUES (?, ?, ?, ?)", date, name, quantity, calories)
	return err
}

type logRow struct {
	name     string
	quantity float64
	calories float64
}

func todayLog(db *sql.DB, date string) ([]logRow, error) {
	rows, err := db.Query("SELECT name, quantity, calories FROM log WHERE date = ? ORDER BY id", date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []logRow
	for rows.Next() {
		var r logRow
		rows.Scan(&r.name, &r.quantity, &r.calories)
		entries = append(entries, r)
	}
	return entries, nil
}

func todayCalories(db *sql.DB, date string) (float64, error) {
	var total float64
	err := db.QueryRow("SELECT COALESCE(SUM(calories), 0) FROM log WHERE date = ?", date).Scan(&total)
	return total, err
}

func upsertWeight(db *sql.DB, date string, lbs float64) error {
	_, err := db.Exec("INSERT OR REPLACE INTO weight (date, lbs) VALUES (?, ?)", date, lbs)
	return err
}

func weightStats(db *sql.DB, date string) (avg float64, today float64, hasToday bool, hasHistory bool, err error) {
	err = db.QueryRow("SELECT lbs FROM weight WHERE date = ?", date).Scan(&today)
	if err == sql.ErrNoRows {
		hasToday = false
		err = nil
	} else if err != nil {
		return
	} else {
		hasToday = true
	}

	rows, err := db.Query("SELECT lbs FROM weight WHERE date <= ? ORDER BY date DESC LIMIT 7", date)
	if err != nil {
		return
	}
	defer rows.Close()
	var sum float64
	var count int
	for rows.Next() {
		var v float64
		rows.Scan(&v)
		sum += v
		count++
	}
	if count > 0 {
		avg = sum / float64(count)
		hasHistory = true
	}
	return
}

func deleteLogByDate(db *sql.DB, date string) error {
	_, err := db.Exec("DELETE FROM log WHERE date = ?", date)
	return err
}

func allFoods(db *sql.DB) ([]food, error) {
	rows, err := db.Query("SELECT name, calories FROM foods ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var foods []food
	for rows.Next() {
		var f food
		rows.Scan(&f.name, &f.calories)
		foods = append(foods, f)
	}
	return foods, nil
}
