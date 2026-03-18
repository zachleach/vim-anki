# statusline — Shared Statusline Component

Renders due card count, review streak, calorie count, and weight stats in both the Claude Code statusline and vim statusline. Queries `~/.personal.db` with a single connection.

## Build (Claude Code statusline)
```bash
cd ~/personal-tools/statusline && CGO_ENABLED=1 go build -o review-statusline . && cp review-statusline ~/.local/bin/
```

## Source Files
- `main.go` — Claude Code statusline Go binary; reads JSON from stdin, queries DB, renders rainbow gradient animation
- `statusline.vim` — vim statusline; vimscript + embedded Python, persistent SQLite connection, 100ms timer refresh, rainbow gradient animation

## Vim statusline
Sourced from `~/.vimrc`: `source ~/personal-tools/statusline/statusline.vim`

### termguicolors + t_Co=16 interaction
The vim statusline enables `termguicolors` for truecolor gradient rendering. This breaks the terminal's automatic bold-brightening behavior: with `t_Co=16` (no `termguicolors`), `cterm=bold` with no explicit foreground causes the terminal to brighten default gray to white. With `termguicolors`, vim controls colors entirely via hex values, so bold text inherits `Normal guifg=#BFBFBF` and stays gray.

`s:translate_highlights()` converts all existing cterm colors to gui equivalents using the mintty palette at source time. For syntax groups loaded lazily (e.g. `htmlBold` from markdown), a deferred `Syntax` autocmd with `timer_start(0, ...)` explicitly sets `htmlBold guifg=#FFFFFF gui=bold`. The timer is necessary because `Syntax` fires before the syntax file finishes loading. The fix is intentionally targeted (not generic); applying white to ALL bold-without-foreground groups would incorrectly change vim UI elements like `ModeMsg` (`-- INSERT --`, `-- VISUAL --`).

## Segments
- **Cards due**: rainbow gradient when >0, dark gray when 0
- **Calories**: green when under 2000, red when at/over 2000
- **Weight**: 7-day rolling average, deviation from average (green = losing, red = gaining)
- **Streak**: bright when reviewed today, dark gray otherwise
