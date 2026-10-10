-- Build diagnostics in the editor. PyxForge sends the diagnostics of its last build here; they
-- are shown with Neovim's own diagnostics (signs, underlines, virtual text) in a namespace of
-- their own, in buffers already open and in files opened later. The next build replaces them.

local M = {}

M.ns = vim.api.nvim_create_namespace("pyxforge_build")

local by_file = {} -- normalised path -> vim.diagnostic items

local windows = vim.fn.has("win32") == 1

local function key(path)
  local p = vim.fs.normalize(vim.fn.fnamemodify(path, ":p"))
  if windows then
    p = p:lower()
  end
  return p
end

local function apply(buf)
  if not vim.api.nvim_buf_is_valid(buf) or vim.bo[buf].buftype ~= "" then
    return
  end
  local name = vim.api.nvim_buf_get_name(buf)
  if name == "" then
    return
  end
  vim.diagnostic.set(M.ns, buf, by_file[key(name)] or {})
end

-- set replaces the build diagnostics. Each item has file, lnum and col (0-based), severity
-- (1 error … 4 hint), message and source.
function M.set(items)
  by_file = {}
  for _, it in ipairs(items or {}) do
    local k = key(it.file)
    by_file[k] = by_file[k] or {}
    table.insert(by_file[k], {
      lnum = it.lnum,
      col = it.col,
      severity = it.severity,
      message = it.message,
      source = it.source,
    })
  end
  vim.diagnostic.reset(M.ns)
  for _, buf in ipairs(vim.api.nvim_list_bufs()) do
    if vim.api.nvim_buf_is_loaded(buf) then
      apply(buf)
    end
  end
end

-- count returns how many build diagnostics a file has, for tests.
function M.count(path)
  return #(by_file[key(path)] or {})
end

vim.api.nvim_create_autocmd({ "BufReadPost", "BufNewFile" }, {
  group = vim.api.nvim_create_augroup("pyxforge_build", { clear = true }),
  callback = function(ev)
    apply(ev.buf)
  end,
})

return M
