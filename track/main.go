package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const TargetCalories = 3000

func main() {
	args := os.Args[1:]

	db, err := openDB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := initDB(db); err != nil {
		fmt.Fprintf(os.Stderr, "error initializing database: %v\n", err)
		os.Exit(1)
	}

	if len(args) == 0 {
		runLog(db)
		return
	}

	if _, err := strconv.ParseFloat(args[0], 64); err == nil {
		runWeight(db, args[0])
		return
	}

	switch args[0] {
	case "add", "edit":
		runEdit(db)
	case "--view":
		runWeekView(db)
	case "--sync":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: track --sync <file>")
			os.Exit(1)
		}
		runSync(db, args[1])
	case "--select":
		runSelect(db)
	case "log":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: track log <name> <quantity>")
			os.Exit(1)
		}
		runDirectLog(db, args[1], args[2])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		os.Exit(1)
	}
}

// fzfSelectFood runs fzf and returns the selected food name, or "" on cancel
func fzfSelectFood(foods []food) string {
	var buf bytes.Buffer
	for _, f := range foods {
		fmt.Fprintf(&buf, "%s\t%s (%d)\n", f.name, f.name, f.calories)
	}

	cmd := exec.Command("fzf",
		"--delimiter=\t",
		"--with-nth=2",
		"--ansi",
		"--no-sort",
		"--reverse",
		"--no-info",
		"--height=40%",
		"--prompt=> ",
		"--pointer= ",
		"--color=bg+:-1",
	)
	cmd.Stdin = &buf
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	line := strings.TrimSpace(string(out))
	parts := strings.SplitN(line, "\t", 2)
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	return parts[0]
}

func findFoodCal(foods []food, name string) int {
	for _, f := range foods {
		if f.name == name {
			return f.calories
		}
	}
	return 0
}

// runSelect prints "name" (calories) to stdout for the bash wrapper to capture
func runSelect(db *sql.DB) {
	foods, err := allFoods(db)
	if err != nil || len(foods) == 0 {
		return
	}
	name := fzfSelectFood(foods)
	if name == "" {
		return
	}
	cal := findFoodCal(foods, name)
	fmt.Printf("\"%s\" (%d)", name, cal)
}

// runDirectLog logs a food entry by name and quantity string
func runDirectLog(db *sql.DB, name, qtyStr string) {
	qty, err := strconv.ParseFloat(qtyStr, 64)
	if err != nil || qty <= 0 {
		fmt.Fprintln(os.Stderr, "invalid quantity")
		os.Exit(1)
	}

	foods, err := allFoods(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading foods: %v\n", err)
		os.Exit(1)
	}

	foodCal := findFoodCal(foods, name)
	if foodCal == 0 {
		fmt.Fprintf(os.Stderr, "unknown food: %s\n", name)
		os.Exit(1)
	}

	cal := qty * float64(foodCal)
	today := time.Now().Format("2006-01-02")

	if err := logEntry(db, today, name, qty, cal); err != nil {
		fmt.Fprintf(os.Stderr, "error logging: %v\n", err)
		os.Exit(1)
	}

	total, _ := todayCalories(db, today)
	fmt.Printf("logged: %d cal (total today: %d / %d)\n", int(cal), int(total), TargetCalories)
}

// runLog is the fallback when no bash wrapper is active (fzf + inline prompt)
func runLog(db *sql.DB) {
	foods, err := allFoods(db)
	if err != nil || len(foods) == 0 {
		fmt.Println("No foods in database. Use 'track add' to add foods.")
		return
	}

	name := fzfSelectFood(foods)
	if name == "" {
		return
	}
	foodCal := findFoodCal(foods, name)

	fmt.Print("quantity: ")
	reader := bufio.NewReader(os.Stdin)
	qtyStr, _ := reader.ReadString('\n')
	qtyStr = strings.TrimSpace(qtyStr)
	qty, err := strconv.ParseFloat(qtyStr, 64)
	if err != nil || qty <= 0 {
		fmt.Fprintln(os.Stderr, "invalid quantity")
		os.Exit(1)
	}

	cal := qty * float64(foodCal)
	today := time.Now().Format("2006-01-02")

	if err := logEntry(db, today, name, qty, cal); err != nil {
		fmt.Fprintf(os.Stderr, "error logging: %v\n", err)
		os.Exit(1)
	}

	total, _ := todayCalories(db, today)
	fmt.Printf("logged: %d cal (total today: %d / %d)\n", int(cal), int(total), TargetCalories)
}

func runWeekView(db *sql.DB) {
	now := time.Now()
	// Find Monday of current week (ISO: Monday=1, Sunday=7)
	weekday := now.Weekday()
	if weekday == 0 {
		weekday = 7
	}
	monday := now.AddDate(0, 0, -int(weekday-1))

	tmpDir, err := os.MkdirTemp("", "date.tmp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	// Generate 7 date files and the week summary
	var weekLines []string
	dates := make([]string, 7)
	for i := 0; i < 7; i++ {
		day := monday.AddDate(0, 0, i)
		date := day.Format("2006-01-02")
		dates[i] = date

		entries, _ := todayLog(db, date)
		total := 0.0
		var lines []string
		for _, e := range entries {
			lines = append(lines, fmt.Sprintf("\"%s\" (%d) %g", e.name, int(e.calories), e.quantity))
			total += e.calories
		}

		// Write date file
		os.WriteFile(filepath.Join(tmpDir, date), []byte(strings.Join(lines, "\n")+"\n"), 0644)

		// Weight for this day
		var weightStr string
		var lbs float64
		if err := db.QueryRow("SELECT lbs FROM weight WHERE date = ?", date).Scan(&lbs); err == nil {
			weightStr = fmt.Sprintf("%.1f", lbs)
		} else {
			weightStr = "???.?"
		}

		weekLines = append(weekLines, fmt.Sprintf("%s (%s) %d cal", date, weightStr, int(total)))
	}

	// Write week summary
	os.WriteFile(filepath.Join(tmpDir, "week"), []byte(strings.Join(weekLines, "\n")+"\n"), 0644)

	// Cursor on today's line (ISO day-of-week)
	dayOfWeek := int(now.Weekday())
	if dayOfWeek == 0 {
		dayOfWeek = 7
	}

	autocmd := fmt.Sprintf("autocmd BufWritePost %s/* silent !track --sync <amatch>", tmpDir)
	cmd := exec.Command("vim",
		"-c", autocmd,
		fmt.Sprintf("+%d", dayOfWeek),
		filepath.Join(tmpDir, "week"),
	)
	cmd.Dir = tmpDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

// runSync syncs a single date file back to the DB
func runSync(db *sql.DB, filePath string) {
	date := filepath.Base(filePath)
	if date == "week" {
		return
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	foods, _ := allFoods(db)
	var entries []logRow
	for _, line := range strings.Split(string(data), "\n") {
		name, _, qty, ok := parseLogLine(line)
		if !ok {
			continue
		}
		cal := float64(findFoodCal(foods, name)) * qty
		entries = append(entries, logRow{
			name:     name,
			quantity: qty,
			calories: cal,
		})
	}

	deleteLogByDate(db, date)
	for _, e := range entries {
		logEntry(db, date, e.name, e.quantity, e.calories)
	}
}

// parseLogLine parses `"name" (calories) quantity` format
func parseLogLine(line string) (string, int, float64, bool) {
	line = strings.TrimSpace(line)
	if line == "" || !strings.HasPrefix(line, "\"") {
		return "", 0, 0, false
	}
	end := strings.Index(line[1:], "\"")
	if end < 0 {
		return "", 0, 0, false
	}
	name := line[1 : end+1]
	rest := strings.TrimSpace(line[end+2:])

	// Parse (calories)
	if !strings.HasPrefix(rest, "(") {
		return "", 0, 0, false
	}
	closeParen := strings.Index(rest, ")")
	if closeParen < 0 {
		return "", 0, 0, false
	}
	cal, err := strconv.Atoi(strings.TrimSpace(rest[1:closeParen]))
	if err != nil {
		return "", 0, 0, false
	}

	// Parse optional quantity (defaults to 1)
	qtyStr := strings.TrimSpace(rest[closeParen+1:])
	qty := 1.0
	if qtyStr != "" {
		qty, err = strconv.ParseFloat(qtyStr, 64)
		if err != nil || qty <= 0 {
			return "", 0, 0, false
		}
	}

	return name, cal, qty, true
}

// runEdit backs up DB, opens all foods in vim, syncs changes back
func runEdit(db *sql.DB) {
	dbPath := trackDBPath()
	backupPath := dbPath + "." + time.Now().Format("2006-01-02")
	if data, err := os.ReadFile(dbPath); err == nil {
		os.WriteFile(backupPath, data, 0644)
	}

	foods, err := allFoods(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading foods: %v\n", err)
		os.Exit(1)
	}

	tmp, err := os.CreateTemp("", "track-edit-*.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating temp file: %v\n", err)
		os.Exit(1)
	}
	for _, f := range foods {
		fmt.Fprintf(tmp, "\"%s\" (%d)\n", f.name, f.calories)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	cmd := exec.Command("vim", tmp.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	edited, err := os.ReadFile(tmp.Name())
	if err != nil {
		return
	}

	newFoods := make(map[string]int)
	for _, line := range strings.Split(string(edited), "\n") {
		name, cal, ok := parseFoodLine(line)
		if !ok {
			continue
		}
		newFoods[name] = cal
	}

	// Handle removed foods
	for _, f := range foods {
		if _, exists := newFoods[f.name]; !exists {
			var count int
			db.QueryRow("SELECT COUNT(*) FROM log WHERE name = ?", f.name).Scan(&count)
			if count > 0 {
				fmt.Printf("warning: skipping deletion of '%s' (has %d log entries)\n", f.name, count)
			} else {
				db.Exec("DELETE FROM foods WHERE name = ?", f.name)
			}
		}
	}

	// Upsert parsed foods
	for name, cal := range newFoods {
		upsertFood(db, name, cal)
	}

	fmt.Printf("foods updated (%d entries)\n", len(newFoods))
}

func runWeight(db *sql.DB, valStr string) {
	lbs, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid weight: %s\n", valStr)
		os.Exit(1)
	}

	today := time.Now().Format("2006-01-02")
	if err := upsertWeight(db, today, lbs); err != nil {
		fmt.Fprintf(os.Stderr, "error saving weight: %v\n", err)
		os.Exit(1)
	}

	avg, _, _, hasHistory, err := weightStats(db, today)
	if err != nil {
		fmt.Printf("%.1f lbs\n", lbs)
		return
	}

	if hasHistory {
		dev := lbs - avg
		sign := "+"
		if dev < 0 {
			sign = ""
		}
		fmt.Printf("%.1f lbs (7d avg: %.1f, %s%.1f)\n", lbs, avg, sign, dev)
	} else {
		fmt.Printf("%.1f lbs\n", lbs)
	}
}

// parseFoodLine parses `"name" (calories)` format
func parseFoodLine(line string) (string, int, bool) {
	line = strings.TrimSpace(line)
	if line == "" || !strings.HasPrefix(line, "\"") {
		return "", 0, false
	}
	end := strings.Index(line[1:], "\"")
	if end < 0 {
		return "", 0, false
	}
	name := line[1 : end+1]
	rest := strings.TrimSpace(line[end+2:])
	if !strings.HasPrefix(rest, "(") || !strings.HasSuffix(rest, ")") {
		return "", 0, false
	}
	cal, err := strconv.Atoi(strings.TrimSpace(rest[1 : len(rest)-1]))
	if err != nil {
		return "", 0, false
	}
	return name, cal, true
}
