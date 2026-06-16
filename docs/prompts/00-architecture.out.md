# Revaluator Architecture

## Overview

`revaluator` is a Neovim plugin that evaluates the expression under the cursor
using a persistent language interpreter and shows the result as inline virtual
text. Pressing the keybind once previews the result; pressing it again commits
the result into the buffer.

Two components:

1. **Neovim plugin** (Lua) — UI, keybinding, buffer parsing, RPC client.
2. **Go server** (one binary per language) — owns a persistent interpreter,
   evaluates expressions, returns results.

```
+------------------+        JSON-RPC over stdio        +-------------------+
|  Neovim (Lua)    | <-------------------------------> |  server-python    |
|  - keymap        |                                   |  - python interp  |
|  - parse line    |                                   |  - eval expr      |
|  - virtual text  |                                   +-------------------+
|  - commit text   |
+------------------+
```

One server process is spawned **per buffer/file**, and the interpreter inside
it is spawned **once** for that process's lifetime.

## Directory Layout

```
nvim-revaluator/
├── Makefile
├── README.md
├── go.mod
├── cmd/
│   └── server/
│       └── main.go              # entrypoint, shared by all language builds
├── internal/
│   ├── rpc/
│   │   ├── protocol.go          # request/response types
│   │   └── server.go            # stdio JSON-RPC loop
│   └── lang/
│       ├── lang.go              # Interpreter interface + registry
│       ├── python.go            # //go:build python
│       └── javascript.go        # //go:build javascript (stub, future)
├── lua/
│   └── revaluator/
│       ├── init.lua             # setup(), public API
│       ├── config.lua           # defaults + user config merge
│       ├── client.lua           # spawn + JSON-RPC over job stdio
│       ├── manager.lua          # per-buffer client lifecycle
│       ├── parser.lua           # expression extraction from buffer
│       └── ui.lua               # virtual text preview + commit
├── plugin/
│   └── revaluator.lua           # plugin guard, default keymap wiring
└── docs/
    └── prompts/
```

## Build & Language Separation

- Each language is a **separate binary** named `server-<lang>` (e.g.
  `server-python`).
- A single `cmd/server/main.go` is compiled with a build tag selecting the
  language implementation. Files in `internal/lang/` use build constraints:
  - `python.go` → `//go:build python`
  - `javascript.go` → `//go:build javascript`
- Exactly one language file compiles per build, registering its `Interpreter`
  implementation into the registry that `main.go` consumes.

### Makefile

```make
GO      ?= go
BINDIR  ?= bin

.PHONY: all python clean

all: python

python:
	$(GO) build -tags python -o $(BINDIR)/server-python ./cmd/server

clean:
	rm -rf $(BINDIR)
```

Adding a language = new `internal/lang/<lang>.go` with its build tag + a new
Makefile rule. No changes to the RPC layer or `main.go`.

## Go Server

### Interpreter interface (`internal/lang/lang.go`)

```go
type Interpreter interface {
    // Start launches the underlying language process. Called once.
    Start(ctx context.Context) error
    // Eval evaluates source and returns the string representation of the result.
    Eval(source string) (string, error)
    // Close terminates the interpreter.
    Close() error
}

// Registered by the build-tagged language file via init().
var Active Interpreter
func Register(i Interpreter) { Active = i }
```

### Python implementation (`internal/lang/python.go`, `//go:build python`)

- Starts `python3 -i` (or `python3 -u` driving a small REPL driver) as a long-
  lived subprocess, kept open for the process lifetime.
- Maintains interpreter state across `Eval` calls so previously evaluated lines
  remain in scope.
- To get a value: evaluate the expression and capture `repr(result)`. Statements
  (assignments, defs) are executed for their side effects and return empty.
- Uses a sentinel/marker written to stdout to delimit output of each evaluation,
  so the Go side knows when a result is complete.
- Dependency budget: stdlib only (`os/exec`, `bufio`, `encoding/json`).

### RPC (`internal/rpc`)

Transport: newline-delimited JSON over **stdin/stdout** (the plugin owns the
process via a Neovim job; stdio avoids ports, sockets, and auth). `stderr` is
used for logging.

```go
// protocol.go
type Request struct {
    ID     int    `json:"id"`
    Method string `json:"method"`     // "eval" | "shutdown"
    Source string `json:"source"`     // full text to evaluate (may be multi-line)
}

type Response struct {
    ID    int    `json:"id"`
    Ok    bool   `json:"ok"`
    Value string `json:"value"`       // repr of result, empty if no value
    Error string `json:"error"`       // error message if Ok == false
}
```

`server.go` runs the read→dispatch→write loop:
1. Read a line, decode `Request`.
2. On `eval`, call `lang.Active.Eval(req.Source)`.
3. Encode `Response`, write line, flush.
4. On `shutdown`, close interpreter and exit.

`main.go` calls `lang.Active.Start(ctx)` once, then `rpc.Serve(os.Stdin, os.Stdout)`.

## Neovim Plugin (Lua)

### config.lua

Defaults, merged with user opts in `setup`:

```lua
{
  keymap = "<A-w>",
  filetypes = { python = "server-python" },  -- filetype -> binary name
  bin_dir = nil,   -- resolved relative to plugin root if nil
  timeout_ms = 5000,
}
```

### manager.lua — per-buffer interpreter lifecycle

- Keeps a table `bufnr -> client`.
- On first eval in a buffer, resolves the binary from the buffer's `filetype`,
  spawns it via `client.lua`, and caches it. **One process per file.**
- Closes the client on `BufUnload`/`BufDelete` (autocmd) and on `VimLeavePre`.

### client.lua — RPC over job stdio

- `vim.fn.jobstart(cmd, { on_stdout, on_stderr, ... })`.
- Sends requests with `vim.fn.chansend`, tracks pending requests by `id`,
  resolves via callback when the matching `Response` line arrives.
- Buffers partial lines from `on_stdout`.

### parser.lua — expression extraction

- Determines the expression to evaluate starting at the current cursor line.
- For Python: collect the current line; if it appears incomplete (open
  brackets/parens, trailing backslash, or open string), extend downward until
  balanced — supporting multi-line expressions.
- Tracks whether the buffer's interpreter has been "primed". On the **first**
  eval for a buffer, the source sent is **all lines before the current line**
  concatenated, then the expression — so prior context is established once.
  Subsequent evals send only the current expression.

### ui.lua — preview and commit

- **Preview:** render the returned value as virtual text at end of the current
  line using an extmark (`nvim_buf_set_extmark` with `virt_text`,
  `virt_text_pos = "eol"`). Not written to the file.
- **State:** remember the last previewed `{bufnr, line, text}`.
- **Commit:** if the keymap is pressed again while a preview is active for the
  current line, clear the extmark and append the text to the end of the line
  via `nvim_buf_set_text`.
- Any cursor move / edit / mode change clears the pending preview.

### init.lua — flow on keypress

```
keypress ->
  if preview active for this line:
      commit text to buffer; clear preview
  else:
      expr = parser.extract(bufnr, cursor_line)
      client = manager.get_or_spawn(bufnr)
      source = parser.build_source(bufnr, cursor_line, expr, client.primed)
      client:eval(source, function(resp)
          if resp.ok and resp.value ~= "" then
              ui.preview(bufnr, cursor_line, resp.value)
          else
              ui.error(resp.error)
          end
      end)
```

## Data Flow Summary

1. User presses `<A-w>` on a line.
2. Plugin extracts the (possibly multi-line) expression.
3. Manager ensures one server process exists for the buffer.
4. First call also sends all preceding lines to prime interpreter state.
5. Server evaluates in the persistent interpreter, returns `repr`.
6. Plugin shows result as inline virtual text.
7. Second `<A-w>` commits the text into the buffer.

## Dependencies

- **Go:** standard library only.
- **Lua:** Neovim built-in APIs only (`jobstart`, extmarks, autocmds). No
  external Lua libraries.
