package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Consume stdin (Claude Code sends JSON)
	json.NewDecoder(os.Stdin).Decode(&map[string]interface{}{})

	nowMs := time.Now().UnixMilli()
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".notes.db")

	// Rainbow colors
	colors := [11]string{
		"\033[38;2;244;120;120m", "\033[38;2;244;147;117m",
		"\033[38;2;244;174;114m", "\033[38;2;244;202;120m",
		"\033[38;2;244;229;125m", "\033[38;2;186;232;126m",
		"\033[38;2;128;236;128m", "\033[38;2;125;199;184m",
		"\033[38;2;122;162;240m", "\033[38;2;178;140;228m",
		"\033[38;2;244;158;198m",
	}

	// Time-based rainbow offset — shifts 1 position per 100ms regardless of polling rate
	offset := int(nowMs / 100) % 11

	// Apply rainbow coloring to text
	rainbow := func(text string) string {
		ci := 0
		var out strings.Builder
		for _, ch := range text {
			if ch == ' ' {
				out.WriteByte(' ')
			} else {
				out.WriteString(colors[(ci+offset)%11])
				out.WriteRune(ch)
				ci++
			}
		}
		out.WriteString("\033[0m")
		return out.String()
	}

	db, err := sql.Open("sqlite3", dbPath+"?mode=ro")
	if err != nil {
		fmt.Println("0 cards due")
		return
	}
	defer db.Close()

	total := 0
	db.QueryRow("SELECT COUNT(*) FROM schedule_info WHERE due_date <= date('now','localtime') AND flagged = 0").Scan(&total)

	dim := "\033[38;2;100;100;100m"
	dot := "·"
	reset := "\033[0m"

	word := "cards"
	if total == 1 {
		word = "card"
	}

	// Compute review streak: consecutive days with at least one review ending at today
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

	var out strings.Builder

	if total == 0 {
		// All dark gray when nothing due
		fmt.Fprintf(&out, "%s0 cards due · %d day streak%s", dim, streak, reset)
	} else {
		// Rainbow for cards, default color for the rest
		out.WriteString(rainbow(fmt.Sprintf("%d %s due", total, word)))
		fmt.Fprintf(&out, " %s %d day streak", dot, streak)
	}

	fmt.Println(out.String())
}
