# review

A decentralized spaced repetition system using vim as the flashcard interface.

## Simplified Explanation

### The Core Idea

Vim can read from stdin. If you pipe text to it, it opens a buffer with that content:

```bash
echo "what is a context-free grammar?" | vim -
```

That's the entire foundation. A flashcard is just text piped to vim.

### Hiding the Answer

The question and answer are piped together, but you only want to see the question first. A vim startup command deletes everything below the first line:

```bash
echo -e "what is a context-free grammar?\n1. terminals and variables\n2. production rules\n3. start symbol" | vim - -c '2,$d'
```

Now you're staring at just the question. The answer is gone from the buffer, but vim remembers it in the undo history.

### Revealing the Answer

Two steps. First, save what you typed below the question into a register so it isn't lost:

```vim
let typed = getline(2, '$')
call setreg('a', typed, 'l')
```

Then revert the buffer to its original state, restoring the full answer, and paste your attempt below a `---` separator:

```vim
earlier 99999h
call append(line('$'), ['', '---', ''])
normal! G"ap
```

Now you can compare what you recalled against the actual answer.

### Recording Your Response

Vim can exit with a specific exit code using `:cq N`. Keybindings map `4` to `:cq 4` (correct) and `1` to `:cq 1` (wrong). The calling program reads the exit code and updates the schedule.

That's the whole loop. Pipe a card to vim, hide the answer, reveal it, exit with a code. A shell loop and a SQLite database turn this into a full spaced repetition system.

## Background

Anki is a flashcard application that uses spaced repetition to help you remember things. You create flashcards with a question and answer, review them by seeing the question and recalling the answer, then rate how well you remembered. Based on your rating, Anki schedules when you'll see that card again—cards you know well appear less frequently, cards you struggle with appear more often.

The problem is that Anki has its own interface for creating and reviewing cards. If you take notes in a text editor like vim, you have to copy and paste content into Anki separately. This project eliminates that friction by bringing spaced repetition to the terminal—your notes are your flashcards, and you create and review them in the same place.

Unlike Anki, this system is decentralized. Flashcard files can live anywhere on your filesystem—in project directories, scattered across topic folders, wherever makes sense. A single SQLite database at `~/.notes.db` tracks scheduling for all of them. There's no central flashcard directory; you just write notes with question markers and the system finds and tracks them.

## Installation

### Requirements

- Go 1.18+ (CGO_ENABLED=1)
- vim
- fzf
- SQLite

### Setup

1. Clone and build:
   ```bash
   git clone <repo-url> ~/review
   cd ~/review && CGO_ENABLED=1 go build -o review . && cp review ~/.local/bin/
   ```

2. Add the vim keybindings to `~/.vimrc` (see [vimrc Configuration](#vimrc-configuration) below)

3. Add auto-tracking to `~/.vimrc`:
   ```vim
   autocmd BufWritePost *.txt silent! call system('review sync ' . shellescape(expand('%:p')))
   ```

4. Run `review` to verify installation

## File Format

Flashcards are plain text files. Questions start with `>` followed by a tab character. The answer follows on subsequent lines until the next `>	` or end of file. Trailing blank lines are trimmed. Everything before the first question marker is freeform notes—ignored by the parser.

The intended workflow is to summarize concepts in your own words, then create question/answer pairs for periodic self-testing:

```
Summary of the concept in my own words explaining the key ideas
and relationships I need to understand for this topic.

---

>	What is X?
...answer explaining X

>	How does Y relate to Z?
...answer explaining the relationship

>	Why is this important?
...answer with reasoning
```

## Scheduling

Spaced repetition works by showing you cards at increasing intervals. When you remember something correctly, you won't see it again for a while. When you forget, you see it again soon. This optimizes for long-term retention—you spend more time on material you're struggling with and less time on material you already know.

This system uses a fixed interval schedule:

```
[0, 1, 3, 7, 14, 28, 56] days
    ↑
```

New cards start at index 1. If you answer correctly on your first review, the card advances to index 2 (3-day interval). If you answer correctly at the 7-day interval, you won't see that card again for 14 days. If you forget at any point, the interval resets to 0 and you review it again immediately.

## Vim Interface

Cards are displayed in vim using stdin mode (`vim -`), which reads text from a pipe into a buffer. The review loop pipes each card's content to vim and uses a startup command to delete everything below the first line—hiding the answer while showing only the question.

To reveal the answer, press `<Space>` or `<Enter>`. This restores the deleted answer text and appends your original notes below a `---` separator, letting you compare the official answer with your personal summary. Help text with available keybindings is appended at the bottom.

To record your response, vim exits with a specific code using `:cq N`. The exit code tells the review binary how you performed:

| Key       | Action                            | Notification    |
|-----------|-----------------------------------|-----------------|
| `<Space>` | Reveal answer with notes appended |                 |
| `<Enter>` | Reveal answer with notes appended |                 |
| `4`       | Correct—advance to next interval  | `due in N days` |
| `1`       | Wrong—reset interval to 0         | `reset`         |
| `-`       | Skip—remains due today            | `skipped`       |
| `f`       | Flag—suspend card from reviews    | `flagged`       |
| `e`       | Edit source file at question line |                 |
| `<C-z>`   | Undo—restore previous card state  | `undone`        |
| `:q`      | Quit session                      |                 |

Notifications flash in the vim tabline for one second after each action.

**Example of revealed answer:**

Before pressing `<Space>` or `<Enter>`, you see only:
```
>	what is the algorithm for converting CFG to CNF
```

After pressing `<Space>` or `<Enter>`:
```
>	what is the algorithm for converting CFG to CNF
1.    add S_0 -> S
2.    remove rhs non-solitary literals
3.    remove rhs with more than 2x symbols
4.    remove rhs with epsilon
5.    remove rhs with 1x symbols

---

add S0 -> S
remove solitary terminals
remove epsilon transitions
remove binary symbols
remove solo symbols

again (1), good (4), flag (f), skip (-), edit (e), undo (ctrl-z)
```

When you press `e`, the review session pauses and opens the source file in vim at the question line. After you save and close vim, the file is re-parsed and the review session continues with your edits applied.

When you press `<C-z>` (ctrl-z), the system restores the previous card's database state and re-displays it. Undo only works within the current session. If there's no history (first card), ctrl-z terminates the review session.

When you press `f`, the card is flagged and suspended from future reviews. Flagged cards are excluded from due counts until unflagged with `review unflag`.

When you answer wrong (`1`), the card is re-queued immediately so you see it again before the session ends.

### vimrc Configuration

These mappings only trigger in stdin mode (`vim -`), so they won't affect normal vim usage:

```vim
function! RevealAnswer()
    " Save what user typed, restore original content (question + answer),
    " then append separator and user's typed answer for comparison
    let typed = getline(2, '$')
    call setreg('a', typed, 'l')
    earlier 99999h
    call append(line('$'), ['', '---', ''])
    if !empty(typed)
        normal! G"ap
    endif
    call append(line('$'), ['', 'again (1), good (4), flag (f), skip (-), edit (e), undo (ctrl-z)'])
    normal! G
endfunction

autocmd StdinReadPost * nnoremap <buffer> 1 :cq 1<CR>
autocmd StdinReadPost * nnoremap <buffer> 4 :cq 4<CR>
autocmd StdinReadPost * nnoremap <buffer> - :cq 3<CR>
autocmd StdinReadPost * nnoremap <buffer> e :cq 2<CR>
autocmd StdinReadPost * nnoremap <buffer> f :cq 6<CR>
autocmd StdinReadPost * nnoremap <buffer> <C-z> :cq 5<CR>
autocmd StdinReadPost * nnoremap <buffer> <Space> :call RevealAnswer()<CR>
autocmd StdinReadPost * nnoremap <buffer> <CR> :call RevealAnswer()<CR>
```

## Dashboard

Running `review` with no arguments launches an fzf file selector showing files with due cards. The header displays the total due count and your review streak. Selecting a file launches a review session.

```
  2 cards due · 15 day streak

  context-free-languages    3
  pumping-lemma             1
  dfa-minimization          1
  grammar                   5
  vocabulary               12
```

### Browse All

`review --all` shows all tracked files—even those with zero due cards—in fzf with a preview panel listing questions. Selecting a file launches custom study (all cards, no scheduling). Display shows due/total counts per file.

## Auto-Tracking

Any `.txt` file with `>	` question markers is automatically tracked when saved in vim. The `BufWritePost` autocmd calls `review sync`, which parses the file and registers its questions in the database. No extra steps—just write notes and save.

```vim
autocmd BufWritePost *.txt silent! call system('review sync ' . shellescape(expand('%:p')))
```

## Flagging

Flag a card during review by pressing `f`. Flagged cards are suspended—they won't appear in review sessions or due counts until unflagged.

```bash
# List all flagged questions
review flagged

# Unflag a question
review unflag "question text here"
```

## Statusline

The vim statusline shows due card count and review streak with a rainbow gradient animation, refreshing every 100ms. A separate Go binary provides the same information in the Claude Code statusline.

Both exclude flagged cards from due counts.

## Usage

```bash
# Launch fzf dashboard (select a file to review)
review

# Review due cards in a file
review file.txt

# Custom study (all cards, no scheduling)
review -f file.txt

# Browse all tracked files
review --all

# Register a file for tracking
review track file.txt

# Reset schedule for a file
review forget file.txt

# Move file and update database paths
review --mv old/path.txt new/path.txt

# List flagged questions
review flagged

# Unflag a question
review unflag "question text"

# JSON output of due counts
review --json

# Dump schedule table
review --list
```

## Workflow Tips

### Using fzf for File Selection

For reviewing flashcards scattered across directories, using fzf with sourced keybindings provides a faster workflow than typing file paths. Install fzf and add its keybindings to your shell:

```bash
# Add to ~/.bashrc (or ~/.zshrc)
source /usr/share/doc/fzf/examples/key-bindings.bash
```

With this setup, you can use `Ctrl-T` to fuzzy find any file:

```bash
review <Ctrl-T>
```

### Vim Hotkeys

These mappings in `~/.vimrc` let you launch custom study directly from the file you're editing:

| Mode   | Key  | Action                                  |
|--------|------|-----------------------------------------|
| Normal | `\r` | Save and custom study all cards in file |
| Visual | `\r` | Save and custom study selected cards    |
