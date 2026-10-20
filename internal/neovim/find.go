package neovim

// FindOptions says how Find and Replace match: the text is always literal (no patterns).
type FindOptions struct {
	MatchCase bool
	WholeWord bool
}

// FindResult is where a search ended: the match the cursor is on, counted from 1, of Total
// matches in the buffer; Current is 0 when nothing matched.
type FindResult struct {
	Found   bool
	Current int
	Total   int
}

// findLua builds Vim's pattern for literal text (\V, with \C or \c, and \< \> for whole words)
// as the search pattern, so the matches are highlighted and n and N in Neovim go on from them,
// and defines the helpers below.
const findLua = `local function pattern(text, case, word)
  local p = "\\V" .. (case and "\\C" or "\\c")
  local t = vim.fn.escape(text, "\\")
  if word then t = "\\<" .. t .. "\\>" end
  return p .. t
end
local function count(pat)
  local ok, c = pcall(vim.fn.searchcount, { pattern = pat, maxcount = 0, timeout = 500 })
  if not ok or type(c) ~= "table" then return 0, 0 end
  return c.current or 0, c.total or 0
end
`

// Find moves the cursor to the next match of text (back: the previous one), wrapping around
// the buffer. With here, a match at the cursor counts: typing in a find field keeps the match
// already found. An empty text clears the highlighting.
func (s *Session) Find(text string, o FindOptions, back, here bool) (FindResult, error) {
	var r FindResult
	err := s.v.ExecLua(findLua+`local text, case, word, back, here = ...
if text == "" then
  vim.fn.setreg("/", "")
  vim.o.hlsearch = false
  return { Found = false, Current = 0, Total = 0 }
end
local pat = pattern(text, case, word)
vim.fn.setreg("/", pat)
vim.o.hlsearch = true
local flags = (back and "b" or "") .. "w" .. (here and not back and "c" or "")
local found = vim.fn.searchpos(pat, flags)[1] ~= 0
local current, total = count(pat)
return { Found = found, Current = found and current or 0, Total = total }`, &r, text, o.MatchCase, o.WholeWord, back, here)
	return r, err
}

// Replace replaces the match at the cursor, if the cursor is on one, with replacement, then
// moves to the next match.
func (s *Session) Replace(text, replacement string, o FindOptions) (FindResult, error) {
	var r FindResult
	err := s.v.ExecLua(findLua+`local text, repl, case, word = ...
local pat = pattern(text, case, word)
local cur = vim.api.nvim_win_get_cursor(0)
local start = vim.fn.searchpos(pat, "cnW")
if start[1] == cur[1] and start[2] == cur[2] + 1 then
  -- The text is one line, so is the match: matchend gives the byte after it.
  local line = vim.api.nvim_buf_get_lines(0, start[1] - 1, start[1], false)[1] or ""
  local stop = vim.fn.matchend(line, pat, start[2] - 1)
  local rl = vim.split(repl, "\n", { plain = true })
  vim.api.nvim_buf_set_text(0, start[1] - 1, start[2] - 1, start[1] - 1, stop, rl)
  if #rl == 1 then
    vim.api.nvim_win_set_cursor(0, { start[1], start[2] - 1 + #rl[1] })
  end
end
vim.fn.setreg("/", pat)
vim.o.hlsearch = true
local found = vim.fn.searchpos(pat, "w")[1] ~= 0
local current, total = count(pat)
return { Found = found, Current = found and current or 0, Total = total }`, &r, text, replacement, o.MatchCase, o.WholeWord)
	return r, err
}

// ReplaceAll replaces every match in the buffer, as one change to undo, and returns how many
// it replaced.
func (s *Session) ReplaceAll(text, replacement string, o FindOptions) (int, error) {
	var n int
	err := s.v.ExecLua(findLua+`local text, repl, case, word = ...
local pat = pattern(text, case, word)
local _, total = count(pat)
if total == 0 then return 0 end
local view = vim.fn.winsaveview()
local sub = vim.fn.escape(repl, "\\/&~"):gsub("\n", "\\r")
vim.cmd("keeppatterns %s/" .. vim.fn.escape(pat, "/") .. "/" .. sub .. "/ge")
vim.fn.winrestview(view)
return total`, &n, text, replacement, o.MatchCase, o.WholeWord)
	return n, err
}
