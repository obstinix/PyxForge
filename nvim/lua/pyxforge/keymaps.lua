-- Keys PyxForge adds inside Neovim. The shell binds only Ctrl+Shift chords (KEYMAP.md); every
-- other key is Neovim's, so conventional editor keys are added here as Neovim mappings.

local map = vim.keymap.set

-- Ctrl+S saves, as in most editors; in insert mode it stays in insert mode. It goes through
-- pyxforge.save (installed by PyxForge when it attaches), which does not write a file that
-- changed on disk under unsaved changes but asks PyxForge to ask the user.
local function save()
  if _G.pyxforge and _G.pyxforge.save then
    _G.pyxforge.save(0)
  else
    vim.cmd.write()
  end
end
map({ "n", "x", "i" }, "<C-s>", save, { desc = "Save" })

-- F9 sets or clears a breakpoint on the cursor's line, as in most editors; Debug Current File
-- and Debug in QEMU stop there.
map({ "n", "i" }, "<F9>", function()
  require("pyxforge.breakpoints").toggle(0)
end, { desc = "Toggle breakpoint" })

-- A click in the gutter (the sign and number columns) also sets or clears the line's breakpoint
-- (#62). The click goes on to move the cursor as usual; the toggle runs after it, outside the
-- mapping, where buffers may be changed.
map({ "n", "i" }, "<LeftMouse>", function()
  local pos = vim.fn.getmousepos()
  local info = pos.winid ~= 0 and vim.fn.getwininfo(pos.winid)[1]
  if info and pos.line > 0 and pos.wincol >= 1 and pos.wincol <= info.textoff then
    vim.schedule(function()
      if vim.api.nvim_win_is_valid(pos.winid) then
        require("pyxforge.breakpoints").toggle(vim.api.nvim_win_get_buf(pos.winid), pos.line)
      end
    end)
  end
  return "<LeftMouse>"
end, { expr = true, desc = "Click; in the gutter, toggle breakpoint" })

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
