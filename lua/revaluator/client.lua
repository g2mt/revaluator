--- JSON-RPC client over a Neovim job's stdio.
---
--- Spawns a server-<lang> binary via vim.fn.jobstart and communicates using
--- newline-delimited JSON over stdin/stdout. Tracks pending requests by id
--- and resolves them via callback when the matching Response arrives.

local M = {}

--- Spawns a server process and returns a client object.
---
--- The client object provides an eval(source, offset, callback) method that
--- sends an "eval" request and invokes the callback with the parsed Response.
---
--- @param bin string path to the server binary
--- @param config table merged plugin config
--- @return table client object with an :eval method
function M.spawn(bin, config)
  -- TODO: jobstart with on_stdout buffering/line-splitting, chansend,
  --       pending-request map, callback dispatch
  return {}
end

return M
