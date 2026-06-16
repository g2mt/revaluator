--- Default configuration values and merge logic.
--- @class revaluator.Config
--- @field keymap string Neovim keymap to trigger eval/commit (default: "<A-w>")
--- @field bin_dir string|nil Directory containing server-<filetype> binaries
--- @field timeout_ms number RPC timeout in milliseconds (default: 5000)

local M = {}

M.defaults = {
  keymap = "<A-w>",
  bin_dir = nil, -- resolved to <plugin_root>/bin if nil
  timeout_ms = 5000,
  debug = false, -- stub mode: no server spawned, eval always returns "test"
  prefix_highlight = false, -- visually highlight the expression prefix on eval
  prefix_hl_group = "Visual", -- highlight group used for the prefix
}

--- Merges user-supplied options on top of defaults.
--- @param user_opts revaluator.Config|nil
--- @return revaluator.Config
function M.merge(user_opts)
  return vim.tbl_deep_extend("force", M.defaults, user_opts or {})
end

return M
