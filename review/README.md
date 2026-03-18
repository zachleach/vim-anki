# review

A decentralized spaced repetition system using vim as the flashcard interface. Plain text files with `>	` question markers can live anywhere on the filesystem. A single Go binary reviews cards in vim, and SQLite tracks scheduling at `~/.personal.db`.

## Usage

```bash
review                    # fzf dashboard: select a file to review
review file.txt           # review due cards in a file
review -f file.txt        # custom study (all cards, no scheduling)
review --all              # browse all tracked files
review track file.txt     # register a file for tracking
review forget file.txt    # reset schedule for a file
review --mv old.txt new.txt  # move file and update DB paths
review flagged            # list flagged questions
review unflag "question"  # unflag a question
review --json             # JSON output of due counts
review --list             # dump schedule table
```

## File Format

```
Summary of the concept in your own words.

---

>	What is X?
...answer explaining X

>	How does Y relate to Z?
...answer explaining the relationship
```

## Build

```bash
cd ~/personal-tools/review && CGO_ENABLED=1 go build -o review . && cp review ~/.local/bin/
```
