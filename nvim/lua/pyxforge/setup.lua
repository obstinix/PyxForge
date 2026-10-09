-- Run by `pyxforge setup editor` in a headless Neovim: builds the Tree-sitter parsers listed
-- in pyxforge-lock.json with the pinned nvim-treesitter. It needs the tree-sitter CLI and a C
-- compiler, and exits with status 1 if a parser could not be built.

local M = {}

local function fail(msg)
  io.stderr:write(msg .. "\n")
  vim.cmd("cquit 1")
end

function M.parsers(list)
  vim.cmd.packadd("nvim-treesitter")
  local ok, ts = pcall(require, "nvim-treesitter")
  if not ok then
    return fail("nvim-treesitter is not installed: " .. tostring(ts))
  end
  local task_ok, err = pcall(function()
    ts.install(list):wait(30 * 60 * 1000)
  end)
  if not task_ok then
    return fail("parser installation failed: " .. tostring(err))
  end
  local missing = {}
  for _, lang in ipairs(list) do
    if not pcall(vim.treesitter.language.add, lang) then
      missing[#missing + 1] = lang
    end
  end
  if #missing > 0 then
    return fail("parsers not available after installation: " .. table.concat(missing, ", "))
  end
  io.stdout:write("\nparsers ready: " .. table.concat(list, ", ") .. "\n")
  vim.cmd("qall!")
end

return M
