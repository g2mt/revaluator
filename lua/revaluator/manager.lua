--- Per-buffer interpreter client lifecycle management.
---
--- Maintains a mapping of bufnr -> client so that each buffer gets exactly
--- one persistent server process. Spawns on first eval and tears down
--- on buffer unload or Vim exit.

local client = require("revaluator.client")

local M = {}

--- @type table<number, table> bufnr -> client table
local clients = {}

--- Locates or spawns the server process for the given buffer.
---
--- Resolves the binary as config.bin_dir/server-<filetype>. On the first
--- call for a buffer, spawns the process via client.lua and caches it.
---
--- @param bufnr number
--- @param config table merged plugin config
--- @return table client object
function M.get_or_spawn(bufnr, config)
  -- Returns the cached client for the buffer, spawning one if needed.
  -- The actual spawning is delegated to client.lua.
  if clients[bufnr] then
    return clients[bufnr]
  end

  local ft = vim.bo[bufnr].filetype
  local bin_dir = config.bin_dir or vim.fn.stdpath("data") .. "/revaluator/bin"
  local bin = bin_dir .. "/server-" .. ft

  local c = client.spawn(bin, config)
  clients[bufnr] = c
  return c
end

--- Tears down the client for a buffer, closing the server process.
--- @param bufnr number
function M.detach(bufnr)
  local c = clients[bufnr]
  if c then
    clients[bufnr] = nil
    c:shutdown()
  end
end

--- Registers autocmds that tear down clients on buffer unload and Vim exit.
--- @param config table merged plugin config
function M.setup_autocmds(config)
  vim.api.nvim_create_autocmd({ "BufUnload", "BufDelete" }, {
    group = vim.api.nvim_create_augroup("RevaluatorManager", { clear = true }),
    callback = function(args)
      M.detach(args.buf)
    end,
  })

  vim.api.nvim_create_autocmd("VimLeavePre", {
    group = vim.api.nvim_create_augroup("RevaluatorManager", { clear = true }),
    callback = function()
      for bufnr, _ in pairs(clients) do
        M.detach(bufnr)
      end
    end,
  })
end

return M
