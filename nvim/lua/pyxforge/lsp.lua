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
  {
    -- Or basedpyright, a fork of pyright with the same protocol that pip installs.
    name = "pyright",
    cmd = { "pyright-langserver", "--stdio" },
    alternatives = { { "basedpyright-langserver", "--stdio" } },
    filetypes = { "python" },
    markers = {
      "pyproject.toml",
      "pyrightconfig.json",
      "setup.py",
      "setup.cfg",
      "requirements.txt",
      ".git",
    },
  },
  {
    name = "typescript-language-server",
    cmd = { "typescript-language-server", "--stdio" },
    filetypes = { "javascript", "javascriptreact", "typescript", "typescriptreact" },
    markers = { "tsconfig.json", "jsconfig.json", "package.json", ".git" },
  },
  {
    -- Eclipse JDT LS keeps its index in a folder of its own for each project (-data).
    name = "jdtls",
    cmd = { "jdtls" },
    filetypes = { "java" },
    markers = { "pom.xml", "build.gradle", "build.gradle.kts", "settings.gradle", ".git" },
    data = true,
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

-- notify sends an event to PyxForge, when PyxForge attached and told us its channel. level
-- and text are for server messages.
local function notify(kind, buf, detail, level, text)
  local chan = vim.g.pyxforge_channel
  if chan then
    pcall(vim.rpcnotify, chan, "pyxforge_lsp", kind, buf, detail, level or 0, text or "")
  end
end
M.notify = notify

-- Server notices (window/showMessage) go to PyxForge's Log rather than Neovim's message line,
-- where several at once raise a hit-enter prompt that swallows the next key.
local default_show = vim.lsp.handlers["window/showMessage"]
local function show_message(err, result, ctx, config)
  if vim.g.pyxforge_channel and result then
    local client = vim.lsp.get_client_by_id(ctx.client_id)
    notify("message", 0, client and client.name or "language server", result.type, result.message)
    return result
  end
  return default_show(err, result, ctx, config)
end
M.show_message = show_message

local function root_for(buf, markers)
  local name = vim.api.nvim_buf_get_name(buf)
  local found = vim.fs.find(markers, { upward = true, path = vim.fs.dirname(name) })[1]
  if found then
    return vim.fs.dirname(found)
  end
  return vim.fn.getcwd()
end

-- command is the first of a server's commands that is installed, with its program resolved
-- to a full path: on Windows that finds npm's .cmd launchers, which a bare name does not start.
local function command(server)
  for _, cmd in ipairs(vim.list_extend({ server.cmd }, server.alternatives or {})) do
    local exe = vim.fn.exepath(cmd[1])
    if exe ~= "" then
      local out = vim.list_slice(cmd)
      out[1] = exe
      return out
    end
  end
  return nil
end
M.command = command

function M.start(buf)
  local server = by_filetype[vim.bo[buf].filetype]
  if not server or vim.api.nvim_buf_get_name(buf) == "" then
    return nil
  end
  local cmd = command(server)
  if not cmd then
    if not reported[server.name] then
      reported[server.name] = true
      notify("missing", buf, server.name)
    end
    return nil
  end
  local root = root_for(buf, server.markers)
  if server.data then
    local key = vim.fn.sha256(root):sub(1, 16)
    vim.list_extend(cmd, { "-data", vim.fn.stdpath("cache") .. "/" .. server.name .. "/" .. key })
  end
  return vim.lsp.start({
    name = server.name,
    cmd = cmd,
    root_dir = root,
    handlers = { ["window/showMessage"] = show_message },
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
