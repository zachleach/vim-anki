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

const TargetCalories = 2000

func main() {
	// Consume stdin (Claude Code sends JSON)
	json.NewDecoder(os.Stdin).Decode(&map[string]interface{}{})

	nowMs := time.Now().UnixMilli()
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".personal.db")

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

	// Compute review streak
	var todayCount int
	db.QueryRow("SELECT COUNT(*) FROM review_log WHERE date(reviewed_at) = date('now','localtime')").Scan(&todayCount)
	reviewedToday := todayCount > 0

	streak := 0
	start := 0
	if !reviewedToday {
		start = 1
	}
	for i := start; ; i++ {
		var count int
		db.QueryRow(
			"SELECT COUNT(*) FROM review_log WHERE date(reviewed_at) = date('now', 'localtime', ?)",
			fmt.Sprintf("-%d days", i),
		).Scan(&count)
		if count == 0 {
			break
		}
		streak++
	}

	// dark gray streak if not reviewed today
	streakDim := !reviewedToday

	// Query track data from same DB
	var calToday float64
	db.QueryRow("SELECT COALESCE(SUM(calories), 0) FROM log WHERE date = date('now','localtime')").Scan(&calToday)

	var weightToday float64
	var hasWeightToday bool
	if err := db.QueryRow("SELECT lbs FROM weight WHERE date = date('now','localtime')").Scan(&weightToday); err == nil {
		hasWeightToday = true
	}

	var weightAvg float64
	var hasWeightData bool
	if rows, err := db.Query("SELECT lbs FROM weight WHERE date <= date('now','localtime') ORDER BY date DESC LIMIT 7"); err == nil {
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
			weightAvg = sum / float64(count)
			hasWeightData = true
		}
	}

	var out strings.Builder

	if total == 0 {
		fmt.Fprintf(&out, "%s0 cards due%s", dim, reset)
	} else {
		out.WriteString(rainbow(fmt.Sprintf("%d %s due", total, word)))
	}

	// Cal segment after cards due
	if calToday > 0 {
		calColor := "\033[38;2;128;236;128m" // light green: under target
		if calToday >= TargetCalories {
			calColor = "\033[38;2;244;120;120m" // red: at/over target
		}
		fmt.Fprintf(&out, " %s%s%s %s%d cal%s", dim, dot, reset, calColor, int(calToday), reset)
	}

	// Weight segment
	if hasWeightData {
		fmt.Fprintf(&out, " %s%s %d lbs%s", dim, dot, int(weightAvg+0.5), reset)
		if hasWeightToday {
			dev := weightToday - weightAvg
			devColor := "\033[38;2;128;236;128m" // green: losing weight
			sign := ""
			if dev >= 0 {
				devColor = "\033[38;2;244;120;120m" // red: gaining weight
				sign = "+"
			}
			fmt.Fprintf(&out, " %s(%s%.1f)%s", devColor, sign, dev, reset)
		}
	}

	// Streak
	if streakDim {
		fmt.Fprintf(&out, " %s%s %d day streak%s", dim, dot, streak, reset)
	} else {
		fmt.Fprintf(&out, " %s %d day streak", dot, streak)
	}

	fmt.Println(out.String())
}
