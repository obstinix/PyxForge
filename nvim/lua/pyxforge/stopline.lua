-- The line a debugger stopped at (#58): an arrow in the sign column and the line tinted, in
-- one buffer at a time. PyxForge opens the file, then calls show; it calls clear when the
-- program runs again or the session ends.

local M = {}

M.ns = vim.api.nvim_create_namespace("pyxforge_stopline")

function M.clear()
  for _, buf in ipairs(vim.api.nvim_list_bufs()) do
    if vim.api.nvim_buf_is_valid(buf) then
      vim.api.nvim_buf_clear_namespace(buf, M.ns, 0, -1)
    end
  end
end

-- show marks line (from 1) of the buffer of path and, in the window showing that buffer, puts
-- the cursor on it. It returns false when no loaded buffer has that file or the line is past
-- its end.
function M.show(path, line)
  M.clear()
  local buf = vim.fn.bufnr(vim.fn.fnamemodify(path, ":p"))
  if
    buf < 0
    or not vim.api.nvim_buf_is_loaded(buf)
    or line < 1
    or line > vim.api.nvim_buf_line_count(buf)
  then
    return false
  end
  vim.api.nvim_buf_set_extmark(buf, M.ns, line - 1, 0, {
    sign_text = "▶",
    sign_hl_group = "PyxForgeStopSign",
    line_hl_group = "PyxForgeStopLine",
    priority = 30, -- above breakpoints' signs, which share the line
  })
  local win = vim.fn.bufwinid(buf)
  if win ~= -1 then
    vim.api.nvim_win_set_cursor(win, { line, 0 })
    vim.api.nvim_win_call(win, function()
      vim.cmd("normal! zz")
    end)
  end
  return true
end

-- current reports the marked buffer's file and line, or nil: for PyxForge's tests.
function M.current()
  for _, buf in ipairs(vim.api.nvim_list_bufs()) do
    local marks = vim.api.nvim_buf_get_extmarks(buf, M.ns, 0, -1, {})
    if marks[1] then
      return vim.api.nvim_buf_get_name(buf), marks[1][2] + 1
    end
  end
  return nil
end

return M
