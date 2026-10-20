-- System clipboard through PyxForge. Neovim's built-in providers need external programs
-- (win32yank, xclip, wl-copy); an embedded Neovim can ask its UI instead, which already has
-- the system clipboard. Neovim 0.9 takes Lua functions in g:clipboard as well (PXF-DEF-029).

local M = {}

-- setup is called by PyxForge after it attaches, with its RPC channel.
function M.setup(chan)
  vim.g.pyxforge_channel = chan
  local function copy(lines, regtype)
    vim.rpcnotify(chan, "pyxforge_clipboard_set", lines, regtype)
  end
  local function paste()
    local ok, got = pcall(vim.rpcrequest, chan, "pyxforge_clipboard_get")
    if ok and type(got) == "table" then
      return got
    end
    return { { "" }, "v" }
  end
  vim.g.clipboard = {
    name = "PyxForge",
    copy = { ["+"] = copy, ["*"] = copy },
    paste = { ["+"] = paste, ["*"] = paste },
    cache_enabled = 0,
  }
  vim.opt.clipboard = "unnamedplus"
  return true
end

return M
