# Architecture: nvim-revaluator

An inline expression evaluator for Neovim that previews results as virtual text
and commits them on a second keypress.

## Data Flow

1. **Keypress** (`<A-w>` default) → `on_keypress()` in `init.lua`
2. If a preview is already shown on the current line → commit the text to the buffer
3. Otherwise:
   - `parser.get_source(bufnr)` collects the full buffer as a string
   - `parser.get_offset()` computes the byte offset of the cursor line
   - `manager.get_or_spawn()` locates or starts the per-buffer server for the current filetype
   - `client:eval(source, offset, callback)` sends a JSON-RPC `eval` request over stdio
4. The server binary:
   - Receives the JSON-RPC request on stdin
   - Passes `source` and `offset` to the language interpreter
   - Returns the repr of the expression (or an error)
5. The callback renders the result via `ui.preview()` as inline virtual text,
   or `ui.error()` on the status line.
6. On cursor move, text change, or mode change → `ui.clear()` removes the preview.

## Protocol

Communication happens over **JSON-RPC via stdio**: the Neovim plugin spawns a server
binary as a child process and exchanges newline-delimited JSON messages on
stdin/stdout. Each request carries an `id` for callback correlation.

### Request

```json
{ "id": 1, "method": "eval", "params": { "source": "<full_buffer>", "offset": 42 } }
```

| Method | Params | Description |
|--------|--------|-------------|
| `eval` | `source` (string), `offset` (number) | Evaluate the expression at the given byte offset |
| `shutdown` | (none) | Gracefully shut down the server |

### Response

```json
{ "id": 1, "value": "3", "error": "" }
```

| Field | Description |
|-------|-------------|
| `id` | Echoes the request id |
| `value` | `repr()` of the evaluated expression, or empty for statements |
| `error` | Human-readable error message, empty on success |

## Server Binaries

Each language gets its own standalone server binary under `bin/server-<filetype>`.
The binary is responsible for everything language-specific: parsing, AST analysis,
interpreter embedding, and incremental state management. The Neovim plugin treats
all servers identically — it spawns `bin/server-<filetype>` and communicates via
the standard JSON-RPC protocol.

Binaries can be written in any language, not just Go. The only requirements are:
- Accept newline-delimited JSON-RPC requests on stdin
- Write newline-delimited JSON-RPC responses to stdout
- Implement the `eval` and `shutdown` methods
- Use the `source`/`offset` params to locate and evaluate the expression around the cursor

Currently provided:

| Binary | Language | Implementation |
|--------|----------|----------------|
| `server-python` | Go (cgo → libpython) | AST-based expression splitting, incremental prefix evaluation |
| `server-javascript` | Placeholder | Stub (Node.js) |

## Go Server Internals

The Go-based servers (`server-python`) use this internal structure:

| File | Responsibility |
|------|---------------|
| `cmd/server/main.go` | Entry point: start interpreter → serve RPC loop |
| `internal/rpc/protocol.go` | `Request`/`Response` struct definitions |
| `internal/rpc/server.go` | JSON-RPC read→dispatch→write loop; methods: `eval`, `shutdown` |
| `internal/lang/lang.go` | `Interpreter` interface + global `Active` var + `Register()` |
| `internal/lang/python.go` | Python interpreter via cgo (build tag: `python`) |
| `internal/lang/python_test.go` | Test suite for Python expression evaluation |

### Interpreter Interface

```go
type Interpreter interface {
    Start(ctx context.Context) error
    Eval(source string, offset int) (string, error)
    Close() error
}
```

Language-specific implementations register themselves in `init()` via `lang.Register()`.
Build tags control which interpreter gets compiled into the binary (currently: `python`).

## Lua Layer (Neovim Plugin)

| File | Responsibility |
|------|---------------|
| `plugin/revaluator.lua` | Plugin guard; auto-loads on startup |
| `lua/revaluator/init.lua` | Public API: `setup()`, keypress handler, autocmds |
| `lua/revaluator/config.lua` | Default config values and merge logic |
| `lua/revaluator/manager.lua` | Per-buffer server lifecycle: resolve `bin/server-<ft>`, spawn, cache, teardown |
| `lua/revaluator/client.lua` | JSON-RPC client over `vim.fn.jobstart` stdio; also provides a debug stub |
| `lua/revaluator/parser.lua` | Extracts full buffer text and cursor byte offset (language-agnostic) |
| `lua/revaluator/ui.lua` | Virtual text preview/commit via extmarks; prefix highlighting |

## Key Design Decisions

1. **Per-buffer persistent server** — each buffer gets one long-running server process,
   keeping interpreter state across multiple evals on consecutive lines
2. **Full buffer sent every request** — the server always receives the complete buffer
   text, not just the current line, so it can find and evaluate prefix statements
3. **Language-agnostic Lua layer** — parser.lua sends raw buffer + byte offset;
   the server (using language-specific AST) handles expression boundary detection
4. **Preview then commit UX** — first keypress shows inline virtual text,
   second inserts it; cursor move/change clears preview automatically
5. **Convention over configuration** — server binaries are discovered by filetype:
   `bin/server-<filetype>`, with a configurable `bin_dir`
6. **Debug mode** — `config.debug = true` bypasses server spawn entirely,
   returning stub `"test"` values for UI development

## Configuration

| Key | Default | Description |
|-----|---------|-------------|
| `keymap` | `"<A-w>"` | Key to trigger eval/commit |
| `bin_dir` | `nil` (resolves to `<plugin>/bin`) | Directory for `server-<filetype>` binaries |
| `timeout_ms` | `5000` | RPC timeout (currently unused by client) |
| `debug` | `false` | Stub mode — no server, always returns `"test"` |
| `prefix_highlight` | `false` | Highlight prefix lines before cursor |
| `prefix_hl_group` | `"Visual"` | Highlight group for prefix |
