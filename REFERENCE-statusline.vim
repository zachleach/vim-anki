" anki_statusline.vim — animated spaced repetition statusline
" Usage: source ~/.claude/anki/statusline/anki_statusline.vim

let s:db = expand('~/.claude/anki/anki.db')

" Mintty palette: index -> hex
let s:palette = {
  \ '0': '#000000', '1': '#BF0000', '2': '#00BF00', '3': '#BFBF00',
  \ '4': '#0000BF', '5': '#BF00BF', '6': '#00BFBF', '7': '#BFBFBF',
  \ '8': '#404040', '9': '#FF4040', '10': '#40FF40', '11': '#FFFF40',
  \ '12': '#6060FF', '13': '#FF40FF', '14': '#40FFFF', '15': '#FFFFFF',
  \ 'Black': '#000000', 'DarkRed': '#BF0000', 'DarkGreen': '#00BF00',
  \ 'DarkYellow': '#BFBF00', 'DarkBlue': '#0000BF', 'DarkMagenta': '#BF00BF',
  \ 'DarkCyan': '#00BFBF', 'Gray': '#BFBFBF', 'Grey': '#BFBFBF',
  \ 'LightGray': '#BFBFBF', 'LightGrey': '#BFBFBF',
  \ 'DarkGray': '#404040', 'DarkGrey': '#404040',
  \ 'LightRed': '#FF4040', 'Red': '#FF4040',
  \ 'LightGreen': '#40FF40', 'Green': '#40FF40',
  \ 'LightYellow': '#FFFF40', 'Yellow': '#FFFF40',
  \ 'LightBlue': '#6060FF', 'Blue': '#6060FF',
  \ 'LightMagenta': '#FF40FF', 'Magenta': '#FF40FF',
  \ 'LightCyan': '#40FFFF', 'Cyan': '#40FFFF',
  \ 'White': '#FFFFFF',
  \ }

" Translate all cterm highlight groups to gui equivalents using mintty palette
function! s:translate_highlights() abort
  let output = execute('hi')
  for line in split(output, "\n")
    " Skip links and cleared groups
    if line =~# 'links to\|cleared'
      continue
    endif
    " Extract group name
    let group = matchstr(line, '^\S\+')
    if empty(group) || group =~# '^Anki'
      continue
    endif
    " Extract ctermfg and ctermbg
    let cfg = matchstr(line, 'ctermfg=\zs\S\+')
    let cbg = matchstr(line, 'ctermbg=\zs\S\+')
    let cmd = ''
    if !empty(cfg) && has_key(s:palette, cfg)
      let cmd .= ' guifg=' . s:palette[cfg]
    endif
    if !empty(cbg) && has_key(s:palette, cbg)
      let cmd .= ' guibg=' . s:palette[cbg]
    endif
    if !empty(cmd)
      silent! exe 'hi ' . group . cmd
    endif
  endfor
  " Ensure Normal has bg set for termguicolors
  hi Normal guifg=#BFBFBF guibg=#000000
endfunction

" Enable truecolor
set termguicolors

" Translate existing cterm colors to gui equivalents
call s:translate_highlights()

" 11-color gradient matching the Go statusline
let s:gradient = [
  \ '#F47878', '#F49375', '#F4AE72', '#F4CA78',
  \ '#F4E57D', '#BAE87E', '#80EC80', '#7DC7B8',
  \ '#7AA2F0', '#B28CE4', '#F49EC6',
  \ ]

let s:tick = 0
let s:due_count = 0
let s:streak = 0

" Define rainbow highlight groups from gradient
for s:i in range(len(s:gradient))
  exe printf('hi AnkiRainbow%d guifg=%s guibg=NONE gui=NONE', s:i, s:gradient[s:i])
endfor
hi AnkiDim guifg=#BFBFBF guibg=NONE gui=NONE
hi AnkiPad guifg=#000000 guibg=#000000 gui=NONE
hi AnkiNormal guifg=NONE guibg=NONE gui=NONE

" Python-based DB access — keeps connection open in-process
python3 << PYEOF
import sqlite3, os, vim

_conn = None

def _get_conn():
    global _conn
    db = os.path.expanduser('~/.claude/anki/anki.db')
    if _conn is None:
        _conn = sqlite3.connect(db)
        _conn.execute("PRAGMA journal_mode=WAL")
    return _conn

def anki_refresh():
    try:
        conn = _get_conn()
        cur = conn.cursor()
        cur.execute("SELECT (SELECT COUNT(*) FROM note WHERE due_date <= date('now','localtime')) + (SELECT COUNT(*) FROM temp_note)")
        due = cur.fetchone()[0]
        streak = 0
        for i in range(366):
            cur.execute("SELECT COUNT(*) FROM review_log WHERE date(reviewed_at) = date('now', ?)", (f'-{i} days',))
            if cur.fetchone()[0] == 0:
                break
            streak += 1
        vim.command(f'let s:due_count = {due}')
        vim.command(f'let s:streak = {streak}')
    except Exception:
        vim.command('let s:due_count = 0')
        vim.command('let s:streak = 0')
PYEOF

function! s:refresh_data() abort
  py3 anki_refresh()
endfunction

function! AnkiStatusline() abort
  let word = s:due_count == 1 ? 'note' : 'notes'
  let due_text = printf('%d %s due', s:due_count, word)
  let streak_text = printf(' · %d day streak', s:streak)

  let result = ''
  let ci = 0
  for char in split(due_text, '\zs')
    if char ==# ' '
      let result .= ' '
    else
      let gi = (ci + s:tick) % len(s:gradient)
      let result .= '%#AnkiRainbow' . gi . '#' . char
      let ci += 1
    endif
  endfor

  return result . '%#AnkiDim#' . streak_text . '%#AnkiNormal#'
endfunction

function! s:on_tick(timer) abort
  let s:tick += 1

  call s:refresh_data()

  if mode() !=# 'c'
    redrawstatus
  endif
endfunction

" Stop any previous timer
if exists('s:timer_id')
  call timer_stop(s:timer_id)
endif

" Initialize
call s:refresh_data()
set laststatus=2
set statusline=%{%AnkiStatusline()%}
let s:timer_id = timer_start(100, function('s:on_tick'), {'repeat': -1})
