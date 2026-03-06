package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type dueFile struct {
	path string
	name string
	due  int
}

type allFile struct {
	path  string
	name  string
	due   int
	total int
}

func getDueFiles(db *sql.DB) []dueFile {
	files := getTrackedFiles(db)
	var result []dueFile
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			continue
		}
		due := countDueInFile(db, f)
		if due == 0 {
			continue
		}
		name := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
		result = append(result, dueFile{path: f, name: name, due: due})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].name < result[j].name
	})
	return result
}

func getAllFiles(db *sql.DB) []allFile {
	files := getTrackedFiles(db)
	var result []allFile
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			continue
		}
		due := countDueInFile(db, f)
		chunks, err := parseChunks(f)
		total := 0
		if err == nil {
			total = len(chunks)
		}
		name := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
		result = append(result, allFile{path: f, name: name, due: due, total: total})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].name < result[j].name
	})
	return result
}

func computeStreak(db *sql.DB) int {
	streak := 0
	for i := 0; ; i++ {
		var count int
		db.QueryRow(
			"SELECT COUNT(*) FROM review_log WHERE date(reviewed_at) = date('now', ?)",
			fmt.Sprintf("-%d days", i),
		).Scan(&count)
		if count == 0 {
			break
		}
		streak++
	}
	return streak
}

func displayDashboard(db *sql.DB) string {
	files := getDueFiles(db)
	if len(files) == 0 {
		fmt.Println("No cards due.")
		return ""
	}

	// total due
	totalDue := 0
	for _, f := range files {
		totalDue += f.due
	}

	// streak
	streak := computeStreak(db)

	// header
	word := "cards"
	if totalDue == 1 {
		word = "card"
	}
	header := fmt.Sprintf("\033[38;2;0;168;10m%d %s due · %d day streak\033[0m", totalDue, word, streak)

	// find max name length for padding
	maxLen := 0
	for _, f := range files {
		if len(f.name) > maxLen {
			maxLen = len(f.name)
		}
	}
	pad := maxLen + 6

	// build fzf input lines: path \t padded_display
	var buf bytes.Buffer
	for _, f := range files {
		display := fmt.Sprintf("%-*s%3d", pad, f.name, f.due)
		fmt.Fprintf(&buf, "%s\t%s\n", f.path, display)
	}

	cmd := exec.Command("fzf",
		"--delimiter=\t",
		"--with-nth=2",
		"--ansi",
		"--no-sort",
		"--reverse",
		"--no-info",
		"--header="+header,
		"--prompt=> ",
		"--pointer= ",
		"--color=fg:-1,bg:-1,hl:-1,fg+:#00e60d,bg+:-1,hl+:#00e60d,pointer:-1,prompt:#00a80a",
	)
	cmd.Stdin = &buf
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	line := strings.TrimSpace(string(out))
	parts := strings.SplitN(line, "\t", 2)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func displayAllDashboard(db *sql.DB) string {
	fmt.Fprint(os.Stderr, "\033[2J\033[H")

	files := getAllFiles(db)
	if len(files) == 0 {
		fmt.Println("No tracked files.")
		return ""
	}

	// totals
	totalDue := 0
	totalCards := 0
	for _, f := range files {
		totalDue += f.due
		totalCards += f.total
	}

	streak := computeStreak(db)

	word := "cards"
	if totalDue == 1 {
		word = "card"
	}
	header := fmt.Sprintf("\033[38;2;0;168;10m%d %s due · %d total · %d day streak\033[0m", totalDue, word, totalCards, streak)

	// find max name length for padding
	maxLen := 0
	for _, f := range files {
		if len(f.name) > maxLen {
			maxLen = len(f.name)
		}
	}
	pad := maxLen + 4

	// build fzf input: path \t display
	var buf bytes.Buffer
	for _, f := range files {
		dueStr := ""
		if f.due > 0 {
			dueStr = fmt.Sprintf("%3d/", f.due)
		} else {
			dueStr = "   "
		}
		display := fmt.Sprintf("%-*s%s%-3d", pad, f.name, dueStr, f.total)
		fmt.Fprintf(&buf, "%s\t%s\n", f.path, display)
	}

	home, _ := os.UserHomeDir()
	previewCmd := fmt.Sprintf(`f="$(echo {} | cut -f1)"; echo "#   $(echo "$f" | sed 's|^%s|~|')"; echo; grep '^>	' "$f" | sed 's/^>	/>   /'`, home)

	cmd := exec.Command("fzf",
		"--delimiter=\t",
		"--with-nth=2",
		"--ansi",
		"--no-sort",
		"--reverse",
		"--no-info",
		"--header="+header,
		"--prompt=> ",
		"--pointer= ",
		"--color=fg:-1,bg:-1,hl:-1,fg+:#00e60d,bg+:-1,hl+:#00e60d,pointer:-1,prompt:#00a80a",
		"--preview="+previewCmd,
		"--preview-window=right:50%:nohidden",
	)
	cmd.Stdin = &buf
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	line := strings.TrimSpace(string(out))
	parts := strings.SplitN(line, "\t", 2)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func getDueJSON(db *sql.DB, path string) string {
	files := getTrackedFiles(db)

	if path != "" {
		abs, _ := filepath.Abs(path)
		info, err := os.Stat(abs)
		if err != nil {
			return `{"total":0,"files":[]}`
		}
		if !info.IsDir() {
			// single file
			due := countDueInFile(db, abs)
			result, _ := json.Marshal(map[string]interface{}{
				"total": due,
				"files": []map[string]interface{}{
					{"path": abs, "due": due},
				},
			})
			return string(result)
		}
		// filter to directory
		var filtered []string
		for _, f := range files {
			if strings.HasPrefix(f, abs+"/") {
				filtered = append(filtered, f)
			}
		}
		files = filtered
	}

	total := 0
	var fileEntries []map[string]interface{}
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			continue
		}
		due := countDueInFile(db, f)
		total += due
		fileEntries = append(fileEntries, map[string]interface{}{
			"path": f,
			"due":  due,
		})
	}

	if fileEntries == nil {
		fileEntries = []map[string]interface{}{}
	}

	result, _ := json.Marshal(map[string]interface{}{
		"total": total,
		"files": fileEntries,
	})
	return string(result)
}
