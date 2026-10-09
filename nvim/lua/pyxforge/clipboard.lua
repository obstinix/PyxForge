-- System clipboard through PyxForge. Neovim's built-in providers need external programs
-- (win32yank, xclip, wl-copy); an embedded Neovim can ask its UI instead, which already has
-- the system clipboard. Needs Neovim 0.10 or newer (function-valued g:clipboard).

local M = {}

-- setup is called by PyxForge after it attaches, with its RPC channel.
function M.setup(chan)
  vim.g.pyxforge_channel = chan
  if vim.fn.has("nvim-0.10") ~= 1 then
    return false
  end
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
