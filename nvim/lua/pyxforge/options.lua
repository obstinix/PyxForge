-- Editor defaults for systems code. Kept small: Neovim's own defaults stand unless a
-- bootloader, kernel or assembly workflow needs something else.

local opt = vim.opt

opt.number = true
opt.signcolumn = "yes" -- diagnostics do not shift the text
opt.cursorline = true
opt.mouse = "a"
opt.undofile = true -- undo history survives restarts, in PyxForge's own state folder
opt.swapfile = false -- PyxForge starts Neovim with -n as well
opt.updatetime = 300
opt.ignorecase = true
opt.smartcase = true
opt.splitright = true
opt.splitbelow = true
opt.scrolloff = 4
opt.termguicolors = true
opt.autoread = true -- files changed on disk reload unless edited here

-- Assembly sources are usually NASM in PyxForge projects.
vim.filetype.add({
  extension = { asm = "nasm", inc = "nasm", ld = "ld", lds = "ld" },
  filename = { ["pyxforge.toml"] = "toml" },
})

-- Terminals show the program's output only: no line numbers or sign column.
vim.api.nvim_create_autocmd("TermOpen", {
  group = vim.api.nvim_create_augroup("pyxforge_terminal", { clear = true }),
  callback = function()
    vim.opt_local.number = false
    vim.opt_local.relativenumber = false
    vim.opt_local.signcolumn = "no"
    vim.opt_local.cursorline = false
  end,
})

-- Re-check files changed outside Neovim (a build, Git) when the window regains focus.
vim.api.nvim_create_autocmd({ "FocusGained", "BufEnter" }, {
  group = vim.api.nvim_create_augroup("pyxforge_autoread", { clear = true }),
  command = "silent! checktime",
})
