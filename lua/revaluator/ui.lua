--- Inline virtual text preview and commit UI.
---
--- Renders evaluation results as virtual text at end-of-line using extmarks.
--- Tracks the last preview so a second keypress commits the text into the
--- buffer instead of previewing again.

local M = {}

--- Namespace for all revaluator extmarks.
local ns = vim.api.nvim_create_namespace("revaluator")

--- @class revaluator.Preview
--- @field bufnr number
--- @field line number 0-indexed line number
--- @field text string the value displayed
--- @field extmark number extmark id

--- @type revaluator.Preview|nil
local active = nil

--- Shows the evaluation result as virtual text at the end of the current line.
---
--- Uses nvim_buf_set_extmark with virt_text and virt_text_pos = "eol".
--- Stores the preview state {bufnr, line, text} so commit() can find it.
---
--- @param bufnr number
--- @param line number 0-indexed line number
--- @param text string the value to display
function M.preview(bufnr, line, text)
  -- Replace any existing preview first.
  M.clear()

  if not vim.api.nvim_buf_is_valid(bufnr) then
    return
  end

  local extmark = vim.api.nvim_buf_set_extmark(bufnr, ns, line, 0, {
    virt_text = { { " = " .. text, "Comment" } },
    virt_text_pos = "eol",
  })

  active = {
    bufnr = bufnr,
    line = line,
    text = text,
    extmark = extmark,
  }
end

--- Commits the currently previewed text into the buffer at the end of the line.
---
--- Clears the preview extmark and appends the text via nvim_buf_set_text.
---
--- @param bufnr number
--- @param line number 0-indexed line number
function M.commit(bufnr, line)
  if not M.is_preview_active(bufnr, line) then
    return
  end

  local text = active.text

  if not vim.api.nvim_buf_is_valid(bufnr) then
    M.clear()
    return
  end

  -- Append the text to the end of the line.
  local current = vim.api.nvim_buf_get_lines(bufnr, line, line + 1, false)[1] or ""
  local col = #current
  vim.api.nvim_buf_set_text(bufnr, line, col, line, col, { text })

  M.clear()
end

--- Clears any active preview (e.g., on cursor move, mode change, or edit).
function M.clear()
  if active then
    if vim.api.nvim_buf_is_valid(active.bufnr) then
      vim.api.nvim_buf_del_extmark(active.bufnr, ns, active.extmark)
    end
    active = nil
  end
end

--- Returns true if a preview is active for the given buffer and line.
---
--- @param bufnr number
--- @param line number 0-indexed
--- @return boolean
function M.is_preview_active(bufnr, line)
  return active ~= nil and active.bufnr == bufnr and active.line == line
end

--- Displays an evaluation error on the status line.
---
--- @param msg string the error message to show
function M.error(msg)
  vim.api.nvim_echo({ { "revaluator: " .. msg, "ErrorMsg" } }, true, {})
end

return M
