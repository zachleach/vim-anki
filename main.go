package main

import (
	"fmt"
	"os"
)

const (
	Quit    = 0
	Wrong   = 1
	Edit    = 2
	Skip    = 3
	Correct = 4
	Undo    = 5
	Flag    = 6
)

var ScheduleIntervals = [7]int{0, 1, 3, 7, 14, 28, 56}

func main() {
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

	syncAllTrackedFiles(db)

	args := os.Args[1:]

	// review --json [path]
	if hasFlag(args, "--json") {
		path := flagArg(args, "--json")
		fmt.Println(getDueJSON(db, path))
		return
	}

	// review flagged
	if len(args) >= 1 && args[0] == "flagged" {
		listFlagged(db)
		return
	}

	// review unflag <question_hash>
	if len(args) >= 2 && args[0] == "unflag" {
		unflagQuestion(db, args[1])
		return
	}

	// review forget <file>
	if len(args) >= 2 && args[0] == "forget" {
		forgetFileSchedule(db, args[1])
		return
	}

	// review track <file>
	if len(args) >= 2 && args[0] == "track" {
		syncFileQuestions(db, args[1])
		fmt.Printf("Tracking %s\n", args[1])
		return
	}

	// review sync <file>
	if len(args) >= 2 && args[0] == "sync" {
		syncFileQuestions(db, args[1])
		return
	}

	// review -f <file>
	if len(args) >= 2 && args[0] == "-f" {
		customStudy(db, args[1])
		return
	}

	// review (no args) — dashboard with fzf
	if len(args) == 0 {
		selected := displayDashboard(db)
		if selected != "" {
			reviewDueQuestions(db, selected)
		}
		return
	}

	// review <file>
	path := args[0]
	if _, err := os.Stat(path); err != nil {
		fmt.Fprintf(os.Stderr, "not found: %s\n", path)
		os.Exit(1)
	}
	reviewDueQuestions(db, path)
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func flagArg(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
