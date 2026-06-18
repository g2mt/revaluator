--- Buffer text and cursor position utilities.
---
--- The Neovim side is intentionally language-agnostic: it sends the full
--- buffer text plus the 0-based cursor line number. The server (using
--- the real language AST) decides the exact expression bounds.

local M = {}

--- Returns the full text of the current buffer as a single string.
--- @param bufnr number
--- @return string
function M.get_source(bufnr)
  return table.concat(vim.api.nvim_buf_get_lines(bufnr, 0, -1, false), "\n")
end

--- Returns the 0-based line number of the current cursor line.
---
--- This is the position sent to the server. The server uses its own AST
--- to find the actual expression boundaries from this anchor point.
---
--- @return number 0-based line number
function M.get_line()
  return vim.fn.line(".") - 1
end

return M
