--- Plugin guard and default keymap wiring.
---
--- This file is sourced by Neovim at startup when the plugin is installed.
--- It guards against double-loading and wires the default keymap to init.lua.

if vim.g.loaded_revaluator == 1 then
  return
end
vim.g.loaded_revaluator = 1

-- Load the plugin module and call setup with defaults.
-- init.lua's setup() is the public entry point; users may also call it
-- directly in their config to override options.
local revaluator = require("revaluator")
revaluator.setup({})
