--- Inline virtual text preview and commit UI.
---
--- Renders evaluation results as virtual text at end-of-line using extmarks.
--- Tracks the last preview so a second keypress commits the text into the
--- buffer instead of previewing again.

local M = {}

--- Shows the evaluation result as virtual text at the end of the current line.
---
--- Uses nvim_buf_set_extmark with virt_text and virt_text_pos = "eol".
--- Stores the preview state {bufnr, line, text} so commit() can find it.
---
--- @param bufnr number
--- @param line number 0-indexed line number
--- @param text string the value to display
function M.preview(bufnr, line, text)
  -- TODO: create extmark with virt_text, store preview state
end

--- Commits the currently previewed text into the buffer at the end of the line.
---
--- Clears the preview extmark and appends the text via nvim_buf_set_text.
---
--- @param bufnr number
--- @param line number 0-indexed line number
function M.commit(bufnr, line)
  -- TODO: clear extmark, append text to end of line
end

--- Clears any active preview (e.g., on cursor move, mode change, or edit).
function M.clear()
  -- TODO: delete extmark, clear preview state
end

--- Returns true if a preview is active for the given buffer and line.
---
--- @param bufnr number
--- @param line number 0-indexed
--- @return boolean
function M.is_preview_active(bufnr, line)
  -- TODO: check stored preview state
  return false
end

return M
