# personal-tools — Go CLI monorepo

Two personal CLI tools (review, track) sharing a unified SQLite database at `~/.personal.db`, with a shared statusline component.

## Build
**IMPORTANT: ALWAYS copy binaries to `~/.local/bin/` after building. The user runs these from PATH, not from the repo directory. If you skip the copy step, your changes won't take effect.**
```bash
cd ~/personal-tools/review && CGO_ENABLED=1 go build -o review . && cp review ~/.local/bin/
cd ~/personal-tools/track && CGO_ENABLED=1 go build -o track . && cp track ~/.local/bin/
cd ~/personal-tools/statusline && CGO_ENABLED=1 go build -o review-statusline . && cp review-statusline ~/.local/bin/
```

## Unified Database: `~/.personal.db`

All tables live in one SQLite file. Both binaries use `CREATE TABLE IF NOT EXISTS`, so whichever runs first creates its tables. On first run, review's `migrateDB()` copies `~/.notes.db` to `~/.personal.db`, then attaches `~/.track.db` and imports its tables. Old DB files are left in place for rollback.

**Review tables:** `schedule_info`, `review_log`, `review_notify`

**Track tables:** `foods`, `log`, `weight`

## Structure

```
review/          # spaced repetition CLI
track/           # calorie and weight tracking CLI
statusline/      # shared statusline (Claude Code + vim)
reference/       # original Python implementation
```

Each subdirectory has its own `CLAUDE.md` with tool-specific documentation, `go.mod`, and `go.sum`. See those files for detailed schemas, CLI usage, and implementation patterns.

## Statusline Integration

Both the Claude Code statusline and vim statusline query `~/.personal.db` for due counts, review streak, calories, and weight. Both exclude flagged questions from due counts (`AND flagged = 0`). The statusline opens a single connection to `~/.personal.db` instead of separate connections to each tool's DB.

### Claude Code statusline
Go binary at `statusline/main.go`, installed to `~/.local/bin/review-statusline`. Reads JSON from stdin, queries DB, renders rainbow gradient animation.

### Vim statusline
Vimscript + embedded Python at `statusline/statusline.vim`. Persistent SQLite connection, 100ms timer refresh, rainbow gradient animation.

Sourced from `~/.vimrc`: `source ~/personal-tools/statusline/statusline.vim`

### termguicolors + t_Co=16 interaction
The vim statusline enables `termguicolors` for truecolor gradient rendering. This breaks the terminal's automatic bold-brightening behavior: with `t_Co=16` (no `termguicolors`), `cterm=bold` with no explicit foreground causes the terminal to brighten default gray to white. With `termguicolors`, vim controls colors entirely via hex values, so bold text inherits `Normal guifg=#BFBFBF` and stays gray.

`s:translate_highlights()` converts all existing cterm colors to gui equivalents using the mintty palette at source time. For syntax groups loaded lazily (e.g. `htmlBold` from markdown), a deferred `Syntax` autocmd with `timer_start(0, ...)` explicitly sets `htmlBold guifg=#FFFFFF gui=bold`. The timer is necessary because `Syntax` fires before the syntax file finishes loading. The fix is intentionally targeted (not generic); applying white to ALL bold-without-foreground groups would incorrectly change vim UI elements like `ModeMsg` (`-- INSERT --`, `-- VISUAL --`).

## Claude Code Skill
The `note` skill at `~/.claude/skills/note/SKILL.md` handles note file organization; renaming, splitting, and combining `.txt` note files. It triggers on anything about notes, note-taking, or organizing note files. The skill proposes changes, waits for approval, then executes and runs `review sync` on each new file.

## Dependencies
- Go 1.18+, CGO_ENABLED=1
- `github.com/mattn/go-sqlite3`
- vim
- fzf
