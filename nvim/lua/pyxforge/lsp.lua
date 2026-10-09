-- Language servers. Each starts when a buffer of its filetype opens, if its executable is
-- installed; a missing server is reported to PyxForge once and never retried in a loop. No
-- plugin is needed: vim.lsp.start is part of Neovim.

local M = {}

M.servers = {
  {
    name = "clangd",
    cmd = { "clangd" },
    filetypes = { "c", "cpp" },
    markers = { "compile_commands.json", ".clangd", "pyxforge.toml", ".git" },
  },
  {
    name = "gopls",
    cmd = { "gopls" },
    filetypes = { "go", "gomod" },
    markers = { "go.mod", ".git" },
  },
  {
    name = "rust-analyzer",
    cmd = { "rust-analyzer" },
    filetypes = { "rust" },
    markers = { "Cargo.toml", ".git" },
  },
  {
    name = "asm-lsp",
    cmd = { "asm-lsp" },
    filetypes = { "nasm", "asm" },
    markers = { ".asm-lsp.toml", "pyxforge.toml", ".git" },
  },
  {
    name = "lua-language-server",
    cmd = { "lua-language-server" },
    filetypes = { "lua" },
    markers = { ".luarc.json", ".git" },
  },
  {
    name = "taplo",
    cmd = { "taplo", "lsp", "stdio" },
    filetypes = { "toml" },
    markers = { "pyxforge.toml", ".git" },
  },
  {
    name = "marksman",
    cmd = { "marksman", "server" },
    filetypes = { "markdown" },
    markers = { ".marksman.toml", ".git" },
  },
  {
    name = "bash-language-server",
    cmd = { "bash-language-server", "start" },
    filetypes = { "sh", "bash" },
    markers = { ".git" },
  },
}

local by_filetype = {}
for _, s in ipairs(M.servers) do
  for _, ft in ipairs(s.filetypes) do
    by_filetype[ft] = s
  end
end

local reported = {}

-- supports works with Neovim 0.11+ (client:supports_method) and older releases.
local function supports(client, method)
  local ok, res = pcall(client.supports_method, client, method)
  if ok and res == true then
    return true
  end
  ok, res = pcall(client.supports_method, method)
  return ok and res == true
end

-- notify sends an event to PyxForge, when PyxForge attached and told us its channel.
local function notify(kind, buf, detail)
  local chan = vim.g.pyxforge_channel
  if chan then
    pcall(vim.rpcnotify, chan, "pyxforge_lsp", kind, buf, detail)
  end
end
M.notify = notify

local function root_for(buf, markers)
  local name = vim.api.nvim_buf_get_name(buf)
  local found = vim.fs.find(markers, { upward = true, path = vim.fs.dirname(name) })[1]
  if found then
    return vim.fs.dirname(found)
  end
  return vim.fn.getcwd()
end

function M.start(buf)
  local server = by_filetype[vim.bo[buf].filetype]
  if not server or vim.api.nvim_buf_get_name(buf) == "" then
    return nil
  end
  if vim.fn.executable(server.cmd[1]) ~= 1 then
    if not reported[server.name] then
      reported[server.name] = true
      notify("missing", buf, server.name)
    end
    return nil
  end
  return vim.lsp.start({
    name = server.name,
    cmd = server.cmd,
    root_dir = root_for(buf, server.markers),
  }, { bufnr = buf })
end

local group = vim.api.nvim_create_augroup("pyxforge_lsp", { clear = true })

vim.api.nvim_create_autocmd("FileType", {
  group = group,
  callback = function(ev)
    M.start(ev.buf)
  end,
})

vim.api.nvim_create_autocmd("LspAttach", {
  group = group,
  callback = function(ev)
    local client = vim.lsp.get_client_by_id(ev.data.client_id)
    if not client then
      return
    end
    notify("attach", ev.buf, client.name)
    if vim.lsp.completion and supports(client, "textDocument/completion") then
      vim.lsp.completion.enable(true, client.id, ev.buf, { autotrigger = true })
    end
    local opts = { buffer = ev.buf }
    vim.keymap.set("n", "gd", vim.lsp.buf.definition, opts)
    vim.keymap.set("n", "K", vim.lsp.buf.hover, opts)
  end,
})

vim.api.nvim_create_user_command("PyxFormat", function()
  vim.lsp.buf.format({ async = false })
end, { desc = "Format the buffer with its language server" })

return M
