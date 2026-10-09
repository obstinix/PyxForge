-- Keys PyxForge adds inside Neovim. The shell binds only Ctrl+Shift chords (KEYMAP.md); every
-- other key is Neovim's, so conventional editor keys are added here as Neovim mappings.

local map = vim.keymap.set

-- Ctrl+S saves, as in most editors; in insert mode it stays in insert mode.
map({ "n", "x" }, "<C-s>", "<Cmd>write<CR>", { desc = "Save" })
map("i", "<C-s>", "<Cmd>write<CR>", { desc = "Save" })

-- Leave terminal mode with Escape twice, keeping a single Escape for programs that need it.
map("t", "<Esc><Esc>", [[<C-\><C-n>]], { desc = "Leave terminal mode" })

-- Diagnostics.
map("n", "[d", function()
  vim.diagnostic.goto_prev()
end, { desc = "Previous diagnostic" })
map("n", "]d", function()
  vim.diagnostic.goto_next()
end, { desc = "Next diagnostic" })
map("n", "<leader>e", vim.diagnostic.open_float, { desc = "Show diagnostic" })
