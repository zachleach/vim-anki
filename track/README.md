# track

Simple calorie counting and weight logging CLI. Foods are stored with calorie counts in SQLite at `~/.personal.db`; logging multiplies quantity by per-unit calories.

## Usage

```bash
track                     # fzf food selector + quantity prompt
track 185                 # log weight in lbs
track add                 # open foods list in vim for editing
track view                # show today's log entries and total
track log "chicken" 2     # direct CLI logging
```

## Food Format (in vim editor)

```
"chicken breast" (165)
"rice" (200)
"banana" (105)
```

## Build

```bash
cd ~/personal-tools/track && CGO_ENABLED=1 go build -o track . && cp track ~/.local/bin/
```
