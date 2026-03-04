package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	Quit    = 0
	Wrong   = 1
	Edit    = 2
	Skip    = 3
	Correct = 4
	Undo    = 5
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

	args := os.Args[1:]

	// review --json [path]
	if hasFlag(args, "--json") {
		path := flagArg(args, "--json")
		fmt.Println(getDueJSON(db, path))
		return
	}

	// review forget <file>
	if len(args) >= 2 && args[0] == "forget" {
		forgetFileSchedule(db, args[1])
		return
	}

	// review track <file>
	if len(args) >= 2 && args[0] == "track" {
		abs, _ := filepath.Abs(args[1])
		trackFile(db, abs)
		fmt.Printf("Tracking %s\n", abs)
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

	// review <file> or review <dir> or review (no args)
	if len(args) == 0 {
		displayDueTree(db, "")
		return
	}

	path := args[0]
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "not found: %s\n", path)
		os.Exit(1)
	}

	if info.IsDir() {
		displayDueTree(db, path)
	} else {
		reviewDueQuestions(db, path)
	}
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
