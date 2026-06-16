--- Public entry point for the revaluator plugin.
---
--- setup() merges user configuration with defaults, registers keymaps and
--- autocmds, and wires together config, manager, client, parser, and ui.

local M = {}

--- Initializes the plugin with optional user configuration overrides.
---
--- Flow on keypress:
---  1. If a preview is active for the current line, commit the text.
---  2. Otherwise, send the full buffer + cursor line offset to the server.
---  3. On response, show the result as inline virtual text (preview).
---
--- @param user_opts revaluator.Config|nil
function M.setup(user_opts)
  -- TODO: merge config, spawn manager autocmds, wire keymap to eval/commit flow
end

return M
