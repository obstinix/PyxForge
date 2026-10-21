-- Breakpoints set in the editor (PXF-FEAT-004). A breakpoint is a line of a file, shown with a
-- sign and kept as an extmark, so it moves with its line as lines are added or removed above
-- it. F9, or Run > Toggle Breakpoint, sets or clears the one on the cursor's line. PyxForge
-- reads them when a debugger starts (M.list), is told of every change, and keeps them for the
-- workspace (M.restore): those of files not open are placed when the file is read, and a file
-- read again from disk keeps its own.

local M = {}

M.ns = vim.api.nvim_create_namespace("pyxforge_breakpoints")

local pending = {} -- normalised path -> { file = path, lines = { … } }, for buffers not loaded
local windows = vim.fn.has("win32") == 1

local function key(path)
  local p = vim.fs.normalize(vim.fn.fnamemodify(path, ":p"))
  if windows then
    p = p:lower()
  end
  return p
end

-- file is the buffer's file, or nil for a buffer that is no file (terminal, help, untitled).
local function file(buf)
  if not vim.api.nvim_buf_is_valid(buf) or vim.bo[buf].buftype ~= "" then
    return nil
  end
  local name = vim.api.nvim_buf_get_name(buf)
  if name == "" then
    return nil
  end
  return vim.fn.fnamemodify(name, ":p")
end

local function place(buf, line)
  vim.api.nvim_buf_set_extmark(buf, M.ns, line - 1, 0, {
    sign_text = "●",
    sign_hl_group = "DiagnosticSignError",
    priority = 20, -- above diagnostics' signs
  })
end

-- lines are a buffer's breakpoints, 1-based, each once, in order.
local function lines(buf)
  local out, seen = {}, {}
  for _, m in ipairs(vim.api.nvim_buf_get_extmarks(buf, M.ns, 0, -1, {})) do
    local l = m[2] + 1
    if not seen[l] then
      seen[l] = true
      out[#out + 1] = l
    end
  end
  table.sort(out)
  return out
end

-- list is every breakpoint, as { { File = path, Lines = { … } }, … } sorted by file: those of
-- loaded buffers where their lines are now, and those of files not open.
function M.list()
  local out, seen = {}, {}
  for _, buf in ipairs(vim.api.nvim_list_bufs()) do
    local f = vim.api.nvim_buf_is_loaded(buf) and file(buf)
    if f then
      seen[key(f)] = true
      local ls = lines(buf)
      if #ls > 0 then
        out[#out + 1] = { File = f, Lines = ls }
      end
    end
  end
  for k, p in pairs(pending) do
    if not seen[k] and #p.lines > 0 then
      out[#out + 1] = { File = p.file, Lines = p.lines }
    end
  end
  table.sort(out, function(a, b)
    return a.File < b.File
  end)
  return out
end

local function changed()
  local chan = vim.g.pyxforge_channel
  if chan then
    pcall(vim.rpcnotify, chan, "pyxforge_breakpoints", M.list())
  end
end

-- toggle sets or clears the breakpoint on a line (default: the cursor's) of a buffer (0: the
-- current one), and returns true when one is set now; nil when the buffer is no file.
function M.toggle(buf, line)
  if buf == nil or buf == 0 then
    buf = vim.api.nvim_get_current_buf()
  end
  if not file(buf) then
    return nil
  end
  line = line or vim.api.nvim_win_get_cursor(0)[1]
  local found = vim.api.nvim_buf_get_extmarks(buf, M.ns, { line - 1, 0 }, { line - 1, -1 }, {})
  local set = #found == 0
  if set then
    place(buf, line)
  else
    for _, m in ipairs(found) do
      vim.api.nvim_buf_del_extmark(buf, M.ns, m[1])
    end
  end
  changed()
  return set
end

-- apply places the pending breakpoints of a buffer just read, past its end dropped.
local function apply(buf)
  local f = file(buf)
  if not f then
    return
  end
  local p = pending[key(f)]
  if not p then
    return
  end
  pending[key(f)] = nil
  vim.api.nvim_buf_clear_namespace(buf, M.ns, 0, -1)
  local count = vim.api.nvim_buf_line_count(buf)
  for _, l in ipairs(p.lines) do
    if l >= 1 and l <= count then
      place(buf, l)
    end
  end
end

-- keep moves a loaded buffer's breakpoints to pending, before it is read again or unloaded.
local function keep(buf)
  local f = vim.api.nvim_buf_is_loaded(buf) and file(buf)
  if f then
    local ls = lines(buf)
    if #ls > 0 then
      pending[key(f)] = { file = f, lines = ls }
    end
  end
end

-- restore replaces every breakpoint with items ({ { File, Lines }, … }), as PyxForge kept them.
function M.restore(items)
  pending = {}
  for _, it in ipairs(items or {}) do
    if it.File and it.Lines then
      pending[key(it.File)] = { file = it.File, lines = it.Lines }
    end
  end
  for _, buf in ipairs(vim.api.nvim_list_bufs()) do
    if vim.api.nvim_buf_is_loaded(buf) and file(buf) then
      vim.api.nvim_buf_clear_namespace(buf, M.ns, 0, -1)
      apply(buf)
    end
  end
end

local group = vim.api.nvim_create_augroup("pyxforge_breakpoints", { clear = true })
-- Reading a file again (Reload from Disk, :edit!) replaces its lines, extmarks and all: keep
-- the breakpoints before, and place them after.
vim.api.nvim_create_autocmd("BufReadPre", {
  group = group,
  callback = function(args)
    keep(args.buf)
  end,
})
vim.api.nvim_create_autocmd("BufReadPost", {
  group = group,
  callback = function(args)
    apply(args.buf)
  end,
})
-- Lines move as the file is edited: once it is saved, PyxForge keeps where they are now.
vim.api.nvim_create_autocmd("BufWritePost", {
  group = group,
  callback = function(args)
    if #lines(args.buf) > 0 then
      changed()
    end
  end,
})
vim.api.nvim_create_autocmd("BufUnload", {
  group = group,
  callback = function(args)
    keep(args.buf)
  end,
})

return M
