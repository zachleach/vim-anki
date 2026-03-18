# track — Calorie and Weight Tracking CLI

Simple calorie counting and weight logging tool. Foods are stored in the database with calorie counts; logging multiplies quantity by per-unit calories.

## Build
```bash
cd ~/personal-tools/track && CGO_ENABLED=1 go build -o track . && cp track ~/.local/bin/
```

## Database
SQLite at `~/.personal.db` (shared with review tool).

```sql
foods (
    name TEXT PRIMARY KEY,
    calories INTEGER NOT NULL
)

log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date TEXT NOT NULL,
    name TEXT NOT NULL,
    quantity REAL NOT NULL,
    calories REAL NOT NULL
)

weight (
    date TEXT PRIMARY KEY,
    lbs REAL NOT NULL
)
```

## CLI
```bash
track                     # fzf food selector + quantity prompt, logs entry
track <number>            # log weight in lbs (e.g., track 185)
track add                 # open foods list in vim for editing
track edit                # same as add
track --view              # open week view in vim (gf into date files, edit, :wqa to sync)
track log <name> <qty>    # direct CLI logging without interactive prompts
track --select            # fzf selector output for bash wrapper integration
```

## Food Format (in vim editor)
```
"food name" (calories)
```

## Source Files
- `main.go` — CLI entry point, fzf food selector, logging, weight tracking
- `db.go` — SQLite schema, CRUD operations

## Log Format (in week view date files)
```
"food name" (per_unit_cal) quantity
```
Per-unit calories are computed as `total_calories / quantity` from the log table. On save, total is recomputed as `per_unit * quantity`. Quantity defaults to 1 if omitted.

## Patterns
- `runWeekView` creates a tmp dir with a `week` summary and 7 date files (Mon-Sun), opens vim on `week` with cursor on today, syncs all date files back to DB on exit
- `runEdit` backs up DB before opening foods in vim; parses `"name" (calories)` format on save
- Foods with log entries cannot be deleted (warning shown, deletion skipped)
- Weight logging shows 7-day rolling average and deviation from average
- Target calories constant: 2000
