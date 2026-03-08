package main

import (
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

func writeNotification(db *sql.DB, msg string) {
	db.Exec(
		"INSERT OR REPLACE INTO review_notify (id, message, created_at) VALUES (1, ?, ?)",
		msg, float64(time.Now().UnixMilli())/1000.0,
	)
}

func clearScreen() {
	cmd := exec.Command("clear")
	cmd.Stdout = os.Stdout
	cmd.Run()
}

type card struct {
	questionLine string
	chunk        Chunk
}

type historyEntry struct {
	question  string
	prevState *ScheduleRow
	card      card
}

func displayInVim(chunk Chunk, name string) int {
	args := []string{"--not-a-term", "-c", "normal ggjdG"}
	if name != "" {
		args = append(args, "-c", fmt.Sprintf("file %s", name))
	}
	args = append(args, "-")

	clearScreen()

	cmd := exec.Command("vim", args...)
	cmd.Stdin = bytes.NewReader([]byte(chunk.FullText))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
	return cmd.ProcessState.ExitCode()
}

func openFileForEdit(filePath, questionLine string) {
	abs, _ := filepath.Abs(filePath)
	data, err := os.ReadFile(abs)
	if err != nil {
		exec.Command("vim", abs).Run()
		return
	}

	lineNum := 0
	for i, line := range strings.Split(string(data), "\n") {
		if line == ">\t"+questionLine {
			lineNum = i + 1
			break
		}
	}

	if lineNum > 0 {
		cmd := exec.Command("vim", "+"+strconv.Itoa(lineNum), "-c", "normal zt", abs)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	} else {
		cmd := exec.Command("vim", abs)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	}
	clearScreen()
}

func getDueCards(db *sql.DB, chunks []Chunk, reviewed map[string]bool) []card {
	var due []card
	for _, c := range chunks {
		if !reviewed[c.QuestionLine] && isDue(db, c.QuestionLine) {
			due = append(due, card{questionLine: c.QuestionLine, chunk: c})
		}
	}
	return due
}

func reviewDueQuestions(db *sql.DB, filePath string) {
	abs, _ := filepath.Abs(filePath)

	// change to file's directory so tmux panes open in context
	os.Chdir(filepath.Dir(abs))

	reviewed := make(map[string]bool)
	var history []historyEntry

	for {
		chunks, err := parseChunks(abs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading file: %v\n", err)
			return
		}

		due := getDueCards(db, chunks, reviewed)
		if len(due) == 0 {
			if len(reviewed) == 0 {
				fmt.Println("No due questions in this file.")
			}
			clearScreen()
			return
		}

		queue := make([]card, len(due))
		copy(queue, due)

		broke := false
		for len(queue) > 0 {
			c := queue[0]
			queue = queue[1:]

			exitCode := displayInVim(c.chunk, filepath.Base(abs))

			switch exitCode {
			case Quit:
				clearScreen()
				return

			case Edit:
				openFileForEdit(abs, c.questionLine)
				broke = true

			case Flag:
				flagQuestionDB(db, c.questionLine, abs)
				reviewed[c.questionLine] = true
				writeNotification(db, "flagged")

			case Undo:
				if len(history) == 0 {
					clearScreen()
					return
				}
				prev := history[len(history)-1]
				history = history[:len(history)-1]

				restoreScheduleState(db, prev.question, prev.prevState)
				delete(reviewed, prev.question)

				// put current card back, then previous card in front
				queue = append([]card{prev.card, c}, queue...)
				writeNotification(db, "undone")

			default: // Wrong, Correct, Skip
				prevState, _ := getScheduleInfo(db, c.questionLine)
				history = append(history, historyEntry{
					question:  c.questionLine,
					prevState: prevState,
					card:      c,
				})

				updateSchedule(db, c.questionLine, abs, exitCode)

				switch exitCode {
				case Wrong:
					writeNotification(db, "reset")
				case Correct:
					currentIndex := 1
					if prevState != nil {
						currentIndex = prevState.ReviewDateIndex
					}
					newIndex := currentIndex + 1
					if newIndex >= len(ScheduleIntervals) {
						newIndex = len(ScheduleIntervals) - 1
					}
					days := ScheduleIntervals[newIndex]
					if days == 1 {
						writeNotification(db, "due in 1 day")
					} else {
						writeNotification(db, fmt.Sprintf("due in %d days", days))
					}
				}

				if exitCode == Wrong {
					queue = append([]card{c}, queue...)
				} else {
					reviewed[c.questionLine] = true
				}
			}

			if broke {
				break
			}
		}

		if !broke {
			clearScreen()
			return
		}
	}
}

func customStudy(db *sql.DB, filePath string) {
	abs, _ := filepath.Abs(filePath)

	reviewed := make(map[string]bool)
	var history []historyEntry

	for {
		chunks, err := parseChunks(abs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading file: %v\n", err)
			return
		}

		var remaining []card
		for _, c := range chunks {
			if !reviewed[c.QuestionLine] {
				remaining = append(remaining, card{questionLine: c.QuestionLine, chunk: c})
			}
		}

		if len(remaining) == 0 {
			if len(reviewed) == 0 {
				fmt.Println("No questions in this file.")
			}
			clearScreen()
			return
		}

		queue := make([]card, len(remaining))
		copy(queue, remaining)

		broke := false
		for len(queue) > 0 {
			c := queue[0]
			queue = queue[1:]

			exitCode := displayInVim(c.chunk, filepath.Base(abs))

			switch exitCode {
			case Quit:
				clearScreen()
				return

			case Edit:
				openFileForEdit(abs, c.questionLine)
				broke = true

			case Undo:
				if len(history) == 0 {
					clearScreen()
					return
				}
				prev := history[len(history)-1]
				history = history[:len(history)-1]
				delete(reviewed, prev.question)
				queue = append([]card{prev.card, c}, queue...)

			default:
				history = append(history, historyEntry{question: c.questionLine, card: c})

				if exitCode == Wrong {
					queue = append([]card{c}, queue...)
				} else {
					reviewed[c.questionLine] = true
				}
			}

			if broke {
				break
			}
		}

		if !broke {
			clearScreen()
			return
		}
	}
}
