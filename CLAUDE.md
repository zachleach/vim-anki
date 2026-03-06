# review — Go spaced repetition CLI

Decentralized spaced repetition system. Plain text files with `>\t` question markers can live anywhere on the filesystem. Single Go binary reviews cards in vim. SQLite tracks scheduling.

## Build
**IMPORTANT: ALWAYS copy the binary to `~/.local/bin/` after building. The user runs `review` from PATH, not from the repo directory. If you skip the copy step, your changes won't take effect.**
```bash
cd ~/review && CGO_ENABLED=1 go build -o review . && cp review ~/.local/bin/
```

## Key Constants
```go
Quit, Wrong, Edit, Skip, Correct, Undo, Flag = 0, 1, 2, 3, 4, 5, 6
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
schedule_info (
    question TEXT PRIMARY KEY,       -- question text (>\t prefix stripped)
    file_path TEXT,
    due_date TEXT NOT NULL,          -- YYYY-MM-DD
    review_date_index INTEGER NOT NULL,
    flagged INTEGER NOT NULL DEFAULT 0
)

review_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    question TEXT,
    reviewed_at TEXT,
    outcome TEXT,                    -- 'correct' or 'wrong'
    review_date_index INTEGER
)
```

## CLI
```bash
review                    # fzf dashboard: select a file to review
review <file>             # review due questions in file (vim interface)
review -f <file>          # custom study (all cards, no DB updates)
review track <file>       # register a file
review sync <file>        # parse + register if file has >	lines (for vim autocmd)
review forget <file>      # reset schedule for file
review flagged            # list all flagged questions
review unflag <question>  # unflag a question
review --json [path]      # JSON output: {"total": N, "files": [...]}
review --list             # dump schedule_info table (sqlite3 -table format)
```

## Vim Review Interface
- Cards piped to `vim -` (stdin mode)
- Startup command `ggjdG` hides answer
- User reveals with Space/Enter (mapped to `:earlier 9999h` + notes append)
- Exit codes via `:cq N` — 0=quit, 1=wrong, 2=edit, 3=skip, 4=correct, 5=undo, 6=flag
- Flag suspends the card (excluded from due counts until unflagged)
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
- `parse.go` — `>\t` file parser, Chunk struct (strips `>\t` prefix before storing)
- `db.go` — SQLite schema, all CRUD operations
- `review.go` — vim display, due review loop, custom study loop
- `fzf-dashboard.go` — fzf dashboard (file selector with due counts, streak), JSON output
- `statusline/main.go` — Claude Code statusline Go binary
- `statusline/statusline.vim` — vim statusline (vimscript + embedded Python)

## Dashboard (fzf)
`review` with no args launches an fzf file selector showing files with due cards. Header shows total due count and review streak. Selected file launches a review session. Requires `fzf` in PATH.

- Display name is filename without extension
- Files sorted alphabetically, due counts right-aligned
- Header: dark green (`#008c08`), selected line: bright green (`#00e60d`), prompt: dark green
- fzf flags: `--ansi --no-sort --reverse --no-info --with-nth=2` (path hidden, used for selection)

## Patterns
- Re-parse file after EDIT (user may modify during session)
- Slice-based queue (prepend for wrong/undo)
- `reviewedHashes` map avoids re-showing answered cards
- `history` slice stores previous DB state for undo
- `syncFileQuestions` handles orphan cleanup scoped to file_path
- `reviewDueQuestions` cds to the file's directory before starting (so tmux panes open in context)
- `isDue` uses local midnight (not UTC) for date comparison — `time.Date()` with `now.Location()`

## Statusline Integration
Claude Code statusline and vim statusline both query `~/.notes.db` for due counts and review streak.

### Claude Code statusline
Go binary at `statusline/main.go`, installed to `~/.local/bin/review-statusline`. Reads JSON from stdin, queries DB, renders rainbow gradient animation.

Rebuild: `cd ~/review/statusline && CGO_ENABLED=1 go build -o review-statusline . && cp review-statusline ~/.local/bin/`

### Vim statusline
Vimscript + embedded Python at `statusline/statusline.vim`. Persistent SQLite connection, 100ms timer refresh, rainbow gradient animation.

Sourced from `~/.vimrc`: `source ~/review/statusline/statusline.vim`

Both statusline queries exclude flagged questions from due counts (`AND flagged = 0`).

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
- fzf
