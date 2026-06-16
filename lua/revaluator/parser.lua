--- Expression boundary and offset detection utilities.
---
--- The Neovim side is intentionally language-agnostic: it sends the full
--- buffer text plus the byte offset of the current cursor line. The Go
--- server (using the real language AST) decides the exact expression bounds.

local M = {}

--- Returns the full text of the current buffer as a single string.
--- @param bufnr number
--- @return string
function M.get_source(bufnr)
  return table.concat(vim.api.nvim_buf_get_lines(bufnr, 0, -1, false), "\n")
end

--- Computes the byte offset of the start of the current cursor line.
---
--- This is the position sent to the server. The server uses its own AST
--- to find the actual expression boundaries from this anchor point.
---
--- @return number byte offset
function M.get_offset()
  local line = vim.fn.line(".") - 1 -- 0-indexed
  local lines = vim.api.nvim_buf_get_lines(0, 0, line, false)
  if #lines == 0 then
    return 0
  end
  -- Sum the byte lengths of all preceding lines, plus one for each newline.
  local offset = 0
  for _, l in ipairs(lines) do
    offset = offset + #l + 1 -- +1 for the "\n" separator
  end
  return offset
end

return M
