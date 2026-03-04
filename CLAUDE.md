# review — Go spaced repetition CLI

Decentralized spaced repetition system. Plain text files with `>\t` question markers can live anywhere on the filesystem. Single Go binary reviews cards in vim. SQLite tracks scheduling.

## Build
```bash
cd ~/review && CGO_ENABLED=1 go build -o review . && cp review ~/.local/bin/
```

## Key Constants
```go
Quit, Wrong, Edit, Skip, Correct, Undo = 0, 1, 2, 3, 4, 5
ScheduleIntervals = [7]int{0, 1, 3, 7, 14, 28, 56}
```

New cards start at index 1. Correct advances index, wrong resets to 0.

## File Format
Questions start with `>\t` (greater-than + tab). Answer follows on subsequent lines until next `>\t` or EOF. Trailing blank lines trimmed. Everything before the first `>\t` is freeform (ignored by parser).

```
notes about the topic in your own words

---

>	question text here
answer line 1
answer line 2

>	next question
next answer
```

## Database
SQLite at `~/.notes.db`.

```sql
tracked_file (
    file_path TEXT PRIMARY KEY,
    created_at TEXT
)

schedule_info (
    question_hash TEXT PRIMARY KEY,  -- SHA-256 of question line
    file_path TEXT,
    due_date TEXT NOT NULL,          -- YYYY-MM-DD
    review_date_index INTEGER NOT NULL
)

review_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    question_hash TEXT,
    reviewed_at TEXT,
    outcome TEXT,                    -- 'correct' or 'wrong'
    review_date_index INTEGER
)
```

## CLI
```bash
review                    # tree of tracked files with due counts
review <file>             # review due questions in file (vim interface)
review -f <file>          # custom study (all cards, no DB updates)
review track <file>       # register a file
review sync <file>        # parse + register if file has >	lines (for vim autocmd)
review forget <file>      # reset schedule for file
review --json [path]      # JSON output: {"total": N, "files": [...]}
```

## Vim Review Interface
- Cards piped to `vim -` (stdin mode)
- Startup command `ggjdG` hides answer
- User reveals with Space/Enter (mapped to `:earlier 9999h` + notes append)
- Exit codes via `:cq N` — 0=quit, 1=wrong, 2=edit, 3=skip, 4=correct, 5=undo
- Wrong re-queues card immediately
- Undo restores previous DB state, re-shows previous card
- Edit opens source file at question line, re-parses on return

## Auto-tracking
Vim `BufWritePost *.txt` autocmd calls `review sync` on save. If the file has `>\t` lines, it gets registered and questions synced into DB. Zero extra steps.

```vim
" in ~/.vimrc
autocmd BufWritePost *.txt silent! call system('review sync ' . shellescape(expand('%:p')))
```

## Source Files
- `main.go` — CLI entry point, constants, arg dispatch
- `parse.go` — `>\t` file parser, SHA-256 hashing, Chunk struct
- `db.go` — SQLite schema, all CRUD operations
- `review.go` — vim display, due review loop, custom study loop
- `tree.go` — tree display with due counts, JSON output

## Patterns
- Re-parse file after EDIT (user may modify during session)
- Slice-based queue (prepend for wrong/undo)
- `reviewedHashes` map avoids re-showing answered cards
- `history` slice stores previous DB state for undo
- `syncFileQuestions` handles orphan cleanup scoped to file_path

## Statusline Integration
Claude Code statusline and vim statusline both query the DB for due counts and review streak.

### Claude Code statusline
Go binary at `~/.claude/anki/statusline/anki-statusline`. Reads JSON from stdin, queries DB, renders rainbow gradient animation. Currently points at `~/.claude/anki/anki.db` — needs updating to `~/.notes.db`.

Rebuild: `cd ~/.claude/anki/statusline && CGO_ENABLED=1 go build -o anki-statusline .`

### Vim statusline
Vimscript + embedded Python at `~/.claude/anki/statusline/anki_statusline.vim`. Persistent SQLite connection, 100ms timer refresh, rainbow gradient animation. Currently points at `~/.claude/anki/anki.db` — needs updating to `~/.notes.db`.

Sourced from `~/.vimrc`: `source ~/.claude/anki/statusline/anki_statusline.vim`

## Reference Files
These are from the original Python-based anki system this project is based on, in `reference/`:
- `reference/REFERENCE-anki.py` — original Python implementation
- `reference/REFERENCE-README.md` — original documentation
- `reference/REFERENCE-statusline.go` — Claude Code statusline (Go binary)
- `reference/REFERENCE-statusline.vim` — vim statusline (vimscript + Python)

## Dependencies
- Go 1.18+, CGO_ENABLED=1
- `github.com/mattn/go-sqlite3`
- vim
