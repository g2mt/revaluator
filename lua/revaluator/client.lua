--- JSON-RPC client over a Neovim job's stdio.
---
--- Spawns a server-<lang> binary via vim.fn.jobstart and communicates using
--- newline-delimited JSON over stdin/stdout. Tracks pending requests by id
--- and resolves them via callback when the matching Response arrives.
---
--- In debug mode (config.debug == true), spawns nothing and returns stub
--- responses.

local M = {}

--- Spawns a server process and returns a client object.
---
--- The client object provides:
---   client:eval(source, offset, callback)
---     Sends an "eval" request and invokes callback(response) on reply.
---     response has fields: id, value, error.
---   client:shutdown()
---     Sends a shutdown request, then SIGTERM. After 5 s, SIGKILL.
---
--- @param bin string path to the server binary
--- @param config table merged plugin config
--- @return table client object with :eval and :shutdown methods
function M.spawn(bin, config)
  if config and config.debug then
    return M._create_stub()
  end

  -- ── shared mutable state between job callbacks and client methods ──────
  local pending = {} --- @type table<number, function>  request id -> callback
  local next_id = 0
  local closed = false
  local job_id = -1

  --- Dispatches a single completed line (JSON object) to its callback.
  local function dispatch_line(line)
    if line == "" then
      return
    end
    local ok, resp = pcall(vim.json.decode, line)
    if not ok or not resp or resp.id == nil then
      return
    end
    local cb = pending[resp.id]
    if cb then
      pending[resp.id] = nil
      -- Schedule into the main loop so callbacks can safely touch buffers.
      vim.schedule(function()
        cb(resp)
      end)
    end
  end

  --- Sends a JSON-RPC request. Returns the request id.
  local function send_request(method, params)
    -- print(vim.inspect(params))
    next_id = next_id + 1
    local id = next_id
    local req = { id = id, method = method, params = params or {} }
    local ok = pcall(vim.fn.chansend, job_id, vim.json.encode(req) .. "\n")
    if not ok then
      return nil
    end
    return id
  end

  -- ── spawn ────────────────────────────────────────────────────────────────
  job_id = vim.fn.jobstart({ bin }, {
    on_stdout = function(_, data)
      for _, line in ipairs(data) do
        dispatch_line(line)
      end
    end,
    on_stderr = function(_, data)
      -- stderr is used for server-side logging; silently discard.
    end,
    on_exit = function(_, code)
      vim.schedule(function()
        for id, cb in pairs(pending) do
          cb({ id = id, value = "", error = "server exited with code " .. tostring(code) })
        end
        pending = {}
      end)
    end,
  })

  if job_id <= 0 then
    -- Spawn failed – return a client that always returns an error.
    return {
      eval = function(_, _, _, callback)
        vim.schedule(function()
          callback({ id = -1, value = "", error = "failed to spawn server: " .. bin })
        end)
      end,
      shutdown = function() end,
    }
  end

  -- ── public client object ─────────────────────────────────────────────────
  return {
    --- Evaluate an expression.
    --- @param source string full buffer text
    --- @param offset number byte offset of cursor line start
    --- @param callback function(response)  response has id, value, error
    eval = function(_, source, offset, callback)
      if closed then
        vim.schedule(function()
          callback({ id = -1, value = "", error = "client closed" })
        end)
        return
      end
      local id = send_request("eval", { source = source, offset = offset })
      if id then
        pending[id] = callback
      else
        vim.schedule(function()
          callback({ id = -1, value = "", error = "failed to send request" })
        end)
      end
    end,

    --- Gracefully shuts down the server process.
    ---
    --- 1. Sends a "shutdown" JSON-RPC request so the server can clean up.
    --- 2. Sends SIGTERM via jobstop.
    --- 3. After 5 seconds, sends SIGKILL if the process is still alive.
    shutdown = function(_)
      if closed then
        return
      end
      closed = true

      -- Graceful shutdown request (best-effort).
      pcall(vim.fn.chansend, job_id, vim.json.encode({ id = -1, method = "shutdown", params = {} }) .. "\n")

      -- SIGTERM.
      pcall(vim.fn.jobstop, job_id)

      -- After 5 s, force SIGKILL.
      vim.defer_fn(function()
        pcall(vim.fn.jobstop, job_id, "kill")
      end, 5000)

      -- Reject any remaining pending callbacks.
      for id, cb in pairs(pending) do
        vim.schedule(function()
          cb({ id = id, value = "", error = "client shut down" })
        end)
      end
      pending = {}
    end,
  }
end

--- Creates a stub client for debug mode.
---
--- Spawns no process. eval() immediately schedules the callback with a
--- fixed test value. shutdown() is a no-op.
---
--- @return table
function M._create_stub()
  local id = 0
  local closed = false
  return {
    eval = function(_, source, offset, callback)
      if closed then
        vim.schedule(function()
          callback({ id = -1, value = "", error = "client closed" })
        end)
        return
      end
      id = id + 1
      vim.schedule(function()
        callback({ id = id, value = "test", error = "" })
      end)
    end,
    shutdown = function(_)
      closed = true
    end,
  }
end

return M
