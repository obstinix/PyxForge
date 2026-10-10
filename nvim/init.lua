-- PyxForge's Neovim configuration. PyxForge starts Neovim with `-u` pointing here and with
-- NVIM_APPNAME=pyxforge, so the user's own Neovim configuration, plugins and data are never
-- read or written. Personal additions go in user.lua inside PyxForge's Neovim config folder
-- (`:echo stdpath("config")`); PyxForge never writes that file.

local root = vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":p:h")
vim.opt.runtimepath:prepend(root)

require("pyxforge.options")
require("pyxforge.keymaps")
require("pyxforge.treesitter")
require("pyxforge.lsp")
require("pyxforge.build")

local user = vim.fn.stdpath("config") .. "/user.lua"
if vim.fn.filereadable(user) == 1 then
  local ok, err = pcall(dofile, user)
  if not ok then
    vim.notify("PyxForge: user.lua failed: " .. tostring(err), vim.log.levels.ERROR)
  end
end
