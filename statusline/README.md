# statusline

Shared statusline component for Claude Code and vim. Renders due card count, review streak, calorie count, and weight stats with a rainbow gradient animation. Queries `~/.personal.db`.

## Components

- `main.go` — Claude Code statusline (Go binary, reads JSON from stdin)
- `statusline.vim` — vim statusline (vimscript + embedded Python, 100ms refresh)

## Build (Claude Code statusline)

```bash
cd ~/personal-tools/statusline && CGO_ENABLED=1 go build -o review-statusline . && cp review-statusline ~/.local/bin/
```

## Vim Setup

```vim
" in ~/.vimrc
source ~/personal-tools/statusline/statusline.vim
```
