" statusline.vim — animated spaced repetition statusline
" Usage: source ~/personal-tools/statusline/statusline.vim

let s:db = expand('~/.personal.db')

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
    if empty(group) || group =~# '^Review'
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

" Fix bold syntax groups after syntax loads (terminal bold-brightens to white)
autocmd Syntax * call timer_start(0, {-> execute('hi htmlBold guifg=#FFFFFF gui=bold | hi htmlBoldItalic guifg=#FFFFFF gui=bold,italic', '')})

" 11-color gradient matching the Go statusline
let s:gradient = [
  \ '#F47878', '#F49375', '#F4AE72', '#F4CA78',
  \ '#F4E57D', '#BAE87E', '#80EC80', '#7DC7B8',
  \ '#7AA2F0', '#B28CE4', '#F49EC6',
  \ ]

let s:tick = 0
let s:due_count = 0
let s:streak = 0
let s:reviewed_today = 0
let s:notify_active = 0
let s:notify_msg = ''
let s:cal_today = 0
let s:weight_avg = 0.0
let s:weight_today = 0.0
let s:weight_has_data = 0
let s:weight_has_today = 0

" Define rainbow highlight groups from gradient
for s:i in range(len(s:gradient))
  exe printf('hi ReviewRainbow%d guifg=%s guibg=NONE gui=NONE', s:i, s:gradient[s:i])
endfor
hi ReviewDim guifg=#BFBFBF guibg=NONE gui=NONE
hi ReviewDark guifg=#646464 guibg=NONE gui=NONE
hi ReviewPad guifg=#000000 guibg=#000000 gui=NONE
hi ReviewNormal guifg=NONE guibg=NONE gui=NONE
hi TrackCal guifg=#80EC80 guibg=NONE gui=NONE

" Python-based DB access — keeps connection open in-process
python3 << PYEOF
import sqlite3, os, time, vim

_conn = None

def _get_conn():
    global _conn
    db = os.path.expanduser('~/.personal.db')
    if _conn is None:
        _conn = sqlite3.connect(db)
        _conn.execute("PRAGMA journal_mode=WAL")
    return _conn

def review_refresh():
    try:
        conn = _get_conn()
        conn.rollback()  # reset read transaction to see external WAL writes
        cur = conn.cursor()
        cur.execute("SELECT COUNT(*) FROM schedule_info WHERE due_date <= date('now','localtime') AND flagged = 0")
        due = cur.fetchone()[0]
        cur.execute("SELECT COUNT(*) FROM review_log WHERE date(reviewed_at) = date('now','localtime')")
        reviewed_today = cur.fetchone()[0] > 0
        streak = 0
        start = 0 if reviewed_today else 1
        for i in range(start, 366):
            cur.execute("SELECT COUNT(*) FROM review_log WHERE date(reviewed_at) = date('now','localtime', ?)", (f'-{i} days',))
            if cur.fetchone()[0] == 0:
                break
            streak += 1
        vim.command(f'let s:due_count = {due}')
        vim.command(f'let s:streak = {streak}')
        vim.command(f'let s:reviewed_today = {1 if reviewed_today else 0}')

        # check for review notification
        cur.execute("SELECT message, created_at FROM review_notify WHERE id = 1")
        row = cur.fetchone()
        if row and time.time() - row[1] < 1.0:
            msg = row[0].replace("'", "''")
            vim.command('let s:notify_active = 1')
            vim.command(f"let s:notify_msg = '{msg}'")
        else:
            vim.command('let s:notify_active = 0')
            vim.command("let s:notify_msg = ''")

        # Track data: calories and weight from same DB
        try:
            cur.execute("SELECT COALESCE(SUM(calories), 0) FROM log WHERE date = date('now','localtime')")
            cal = cur.fetchone()[0]
            vim.command(f'let s:cal_today = {int(cal)}')
        except Exception:
            vim.command('let s:cal_today = 0')

        try:
            cur.execute("SELECT lbs FROM weight WHERE date = date('now','localtime')")
            wrow = cur.fetchone()
            has_today = wrow is not None
            weight_today = wrow[0] if has_today else 0.0

            cur.execute("SELECT lbs FROM weight WHERE date <= date('now','localtime') ORDER BY date DESC LIMIT 7")
            wrows = cur.fetchall()
            if wrows:
                avg = sum(r[0] for r in wrows) / len(wrows)
                vim.command(f'let s:weight_avg = {avg:.1f}')
                vim.command(f'let s:weight_today = {weight_today:.1f}')
                vim.command('let s:weight_has_data = 1')
                vim.command(f'let s:weight_has_today = {1 if has_today else 0}')
            else:
                vim.command('let s:weight_has_data = 0')
                vim.command('let s:weight_has_today = 0')
        except Exception:
            vim.command('let s:weight_has_data = 0')
            vim.command('let s:weight_has_today = 0')
    except Exception:
        vim.command('let s:due_count = 0')
        vim.command('let s:streak = 0')
        vim.command('let s:reviewed_today = 0')
        vim.command('let s:notify_active = 0')
        vim.command("let s:notify_msg = ''")
        vim.command('let s:cal_today = 0')
        vim.command('let s:weight_has_data = 0')
        vim.command('let s:weight_has_today = 0')
PYEOF

function! s:refresh_data() abort
  py3 review_refresh()
endfunction

function! ReviewTabline() abort
  return s:notify_msg
endfunction

function! ReviewStatusline() abort
  let word = s:due_count == 1 ? 'card' : 'cards'
  let due_text = printf('%d %s due', s:due_count, word)
  let streak_text = printf(' · %d day streak', s:streak)

  let result = ''

  if s:due_count == 0
    let result .= '%#ReviewDark#' . due_text
  else
    let ci = 0
    for char in split(due_text, '\zs')
      if char ==# ' '
        let result .= ' '
      else
        let gi = (ci + s:tick) % len(s:gradient)
        let result .= '%#ReviewRainbow' . gi . '#' . char
        let ci += 1
      endif
    endfor
  endif

  " Cal segment after cards due
  if s:cal_today > 0
    if s:cal_today >= 2000
      hi TrackCal guifg=#F47878 guibg=NONE gui=NONE
    else
      hi TrackCal guifg=#80EC80 guibg=NONE gui=NONE
    endif
    let result .= '%#ReviewDark# · %#TrackCal#' . s:cal_today . ' cal'
  endif

  " Weight segment
  if s:weight_has_data
    let avg_rounded = float2nr(round(s:weight_avg))
    let result .= '%#ReviewDark# · ' . avg_rounded . ' lbs'
    if s:weight_has_today
      let dev = s:weight_today - s:weight_avg
      if dev >= 0
        hi TrackWeightDev guifg=#F47878 guibg=NONE gui=NONE
        let result .= ' %#TrackWeightDev#(+' . printf('%.1f', dev) . ')'
      else
        hi TrackWeightDev guifg=#80EC80 guibg=NONE gui=NONE
        let result .= ' %#TrackWeightDev#(' . printf('%.1f', dev) . ')'
      endif
    endif
  endif

  if s:reviewed_today
    let result .= '%#ReviewDim#' . streak_text
  else
    let result .= '%#ReviewDark#' . streak_text
  endif

  return result . '%#ReviewNormal#'
endfunction

function! s:on_tick(timer) abort
  let s:tick += 1

  call s:refresh_data()

  " show/hide tabline for notifications
  if s:notify_active && s:notify_msg !=# ''
    if &showtabline != 2
      set showtabline=2
      set tabline=%!ReviewTabline()
    endif
  else
    if &showtabline != 0
      set showtabline=0
    endif
  endif

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
set showtabline=0
set statusline=%{%ReviewStatusline()%}
let s:timer_id = timer_start(100, function('s:on_tick'), {'repeat': -1})
