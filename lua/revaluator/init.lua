--- Public entry point for the revaluator plugin.
---
--- setup() merges user configuration with defaults, registers keymaps and
--- autocmds, and wires together config, manager, client, parser, and ui.

local config = require("revaluator.config")
local manager = require("revaluator.manager")
local parser = require("revaluator.parser")
local ui = require("revaluator.ui")

local M = {}

--- @type revaluator.Config|nil
M.config = nil

--- Handles a keypress: commit an active preview, or evaluate the current line.
---
--- Flow:
---  1. If a preview is active for the current line, commit the text.
---  2. Otherwise, send the full buffer + cursor line offset to the server.
---  3. On response, show the result as inline virtual text (preview).
local function on_keypress()
  local bufnr = vim.api.nvim_get_current_buf()
  local line = vim.fn.line(".") - 1 -- 0-indexed

  -- Second press on the same line commits the active preview.
  if ui.is_preview_active(bufnr, line) then
    ui.commit(bufnr, line)
    return
  end

  local source = parser.get_source(bufnr)
  local offset = parser.get_offset()
  local client = manager.get_or_spawn(bufnr, M.config)

  client:eval(source, offset, function(resp)
    vim.schedule(function()
      if resp and resp.error and resp.error ~= "" then
        ui.error(resp.error)
      elseif resp and resp.value and resp.value ~= "" then
        ui.preview(bufnr, line, resp.value)
      end
    end)
  end)
end

--- Registers autocmds that clear a pending preview on cursor move, edit,
--- or mode change.
local function setup_preview_clear_autocmds()
  local group = vim.api.nvim_create_augroup("RevaluatorPreview", { clear = true })
  vim.api.nvim_create_autocmd(
    { "CursorMoved", "CursorMovedI", "TextChanged", "TextChangedI", "InsertEnter", "ModeChanged" },
    {
      group = group,
      callback = function()
        ui.clear()
      end,
    }
  )
end

--- Initializes the plugin with optional user configuration overrides.
---
--- @param user_opts revaluator.Config|nil
function M.setup(user_opts)
  M.config = config.merge(user_opts)

  manager.setup_autocmds(M.config)
  setup_preview_clear_autocmds()

  vim.keymap.set("n", M.config.keymap, on_keypress, {
    desc = "Revaluator: preview / commit expression result",
    silent = true,
  })
end

return M
