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
}

--- Merges user-supplied options on top of defaults.
--- @param user_opts revaluator.Config|nil
--- @return revaluator.Config
function M.merge(user_opts)
  return vim.tbl_deep_extend("keep", user_opts or {}, M.defaults)
end

return M
