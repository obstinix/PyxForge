-- How files are read and written, so that saving never changes bytes the user did not edit.
--
-- A file is binary when its first 8000 bytes hold a NUL byte and it does not start with a
-- Unicode byte-order mark (UTF-16 and UTF-32 text is full of NULs). This is Git's rule, and
-- PyxForge's explorer uses the same one (internal/textfile). However a binary file is opened
-- here (the explorer's "Open as Text", :e, the jumplist), it is read in 'binary' mode, so
-- no line ending is converted or added, and it is read-only, so :w refuses until the user
-- says otherwise with :w! or :set noreadonly.

local M = {}

local SNIFF = 8000

local boms = { "\239\187\191", "\255\254", "\254\255", "\0\0\254\255" }

--- Reports whether the file at path looks binary; false when it cannot be read.
function M.is_binary(path)
  local fd = vim.uv.fs_open(path, "r", 0)
  if not fd then
    return false
  end
  local data = vim.uv.fs_read(fd, SNIFF, 0) or ""
  vim.uv.fs_close(fd)
  for _, bom in ipairs(boms) do
    if data:sub(1, #bom) == bom then
      return false
    end
  end
  return data:find("\0", 1, true) ~= nil
end

-- Saving a file keeps whether it ended with a line break: a file without one is not given
-- one (EditorConfig's insert_final_newline can still ask for it).
vim.opt.fixendofline = false

local group = vim.api.nvim_create_augroup("pyxforge_files", { clear = true })

vim.api.nvim_create_autocmd("BufReadPre", {
  group = group,
  callback = function(args)
    if M.is_binary(args.match) then
      vim.b[args.buf].pyxforge_binary = true
      vim.bo[args.buf].binary = true -- set before the read, as `nvim -b` does
    end
  end,
})

vim.api.nvim_create_autocmd("BufReadPost", {
  group = group,
  callback = function(args)
    if vim.b[args.buf].pyxforge_binary then
      vim.bo[args.buf].readonly = true
    end
  end,
})

return M
