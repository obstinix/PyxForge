-- Tree-sitter highlighting. Neovim ships parsers for C, Lua, Markdown, Vimscript and queries;
-- `pyxforge setup editor` installs more (Go, Rust, NASM, TOML, Make, linker scripts…) through
-- the pinned nvim-treesitter. A buffer without a parser keeps regex syntax highlighting.

pcall(vim.cmd.packadd, "nvim-treesitter")

local M = {}

-- start enables Tree-sitter for a buffer if a parser for its filetype is installed.
function M.start(buf)
  local ft = vim.bo[buf].filetype
  if ft == "" then
    return false
  end
  local lang = vim.treesitter.language.get_lang and vim.treesitter.language.get_lang(ft) or ft
  if not lang then
    return false
  end
  local ok = pcall(vim.treesitter.start, buf, lang)
  return ok
end

vim.api.nvim_create_autocmd("FileType", {
  group = vim.api.nvim_create_augroup("pyxforge_treesitter", { clear = true }),
  callback = function(ev)
    M.start(ev.buf)
  end,
})

return M
