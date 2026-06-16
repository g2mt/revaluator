# Revaluator Architecture

## Overview

`revaluator` is a Neovim plugin that evaluates the expression under the cursor
using a persistent language interpreter and shows the result as inline virtual
text. Pressing the keybind once previews the result; pressing it again commits
the result into the buffer.

Two components:

1. **Neovim plugin** (Lua) — UI, keybinding, buffer handling, RPC client.
2. **Go server** (one binary per language) — owns a persistent, embedded
   language interpreter, parses and evaluates expressions, returns results.

```
+------------------+        JSON-RPC over stdio        +-------------------+
|  Neovim (Lua)    | <-------------------------------> |  server-python    |
|  - keymap        |                                   |  - libpython       |
|  - send buffer   |                                   |  - parse AST       |
|  - virtual text  |                                   |  - eval expr      |
|  - commit text   |                                   +-------------------+
+------------------+
```

One server process is spawned **per buffer/file**, and the interpreter inside
it is spawned **once** for that process's lifetime.

## Directory Layout

```
nvim-revaluator/
├── Makefile                          [EASY]
├── go.mod                            [EASY]
├── cmd/
│   └── server/
│       └── main.go              # entrypoint, shared by all language builds  [EASY]
├── internal/
│   ├── rpc/
│   │   ├── protocol.go          # request/response types  [EASY]
│   │   └── server.go            # stdio JSON-RPC loop  [HARD]
│   └── lang/
│       ├── lang.go              # Interpreter interface + registry  [EASY]
│       ├── python.go            # //go:build python  (cgo + libpython)  [HARD]
│       └── javascript.go        # //go:build javascript (stub, future)  [EASY]
├── lua/
│   └── revaluator/
│       ├── init.lua             # setup(), public API  [HARD]
│       ├── config.lua           # defaults + user config merge  [EASY]
│       ├── client.lua           # spawn + JSON-RPC over job stdio  [HARD]
│       ├── manager.lua          # per-buffer client lifecycle  [EASY]
│       ├── parser.lua           # current-expression / offset detection  [EASY]
│       └── ui.lua               # virtual text preview + commit  [HARD]
├── plugin/
│   └── revaluator.lua           # plugin guard, default keymap wiring  [EASY]
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
- Each language **must use that language's own interpreter library** for both
  parsing and evaluation. Python uses **libpython** (embedded via cgo); there is
  no hand-written parser. Parsing produces the language's native AST.

### Makefile

```make
GO      ?= go
BINDIR  ?= bin

.PHONY: all python clean

all: python

# python build requires cgo + libpython dev headers/libs
python:
	CGO_ENABLED=1 $(GO) build -tags python -o $(BINDIR)/server-python ./cmd/server

clean:
	rm -rf $(BINDIR)
```

Adding a language = new `internal/lang/<lang>.go` with its build tag, linking
that language's interpreter library + a new Makefile rule. No changes to the RPC
layer or `main.go`.

## Go Server

### Interpreter interface (`internal/lang/lang.go`)

```go
type Interpreter interface {
    // Start initializes the embedded interpreter. Called once.
    Start(ctx context.Context) error

    // Eval evaluates the expression located at byte `offset` within source.
    //   - source is the full buffer text.
    //   - offset is the byte position of the start of the current line.
    // The implementation evaluates everything PRIOR to offset to establish
    // state, then evaluates the current expression and returns its result.
    Eval(source string, offset int) (string, error)

    // Close terminates the interpreter.
    Close() error
}

// Registered by the build-tagged language file via init().
var Active Interpreter
func Register(i Interpreter) { Active = i }
```

### Python implementation (`internal/lang/python.go`, `//go:build python`)

- Embeds **libpython** via cgo and initializes one interpreter for the process
  lifetime (`Py_Initialize` on `Start`, single `__main__` namespace reused
  across calls).
- All parsing uses libpython itself: source is compiled into Python's **AST**
  (via the `ast`/`compile` machinery). Note that **parsing strips comments** —
  the AST contains no comment nodes.
- `Eval(source, offset)` performs:
  1. Split `source` into the **prefix** (bytes `[0, exprStart)`) and the
     **current expression** beginning at the current line.
  2. Use the AST to find where the current expression actually begins (the
     statement/expression whose start lies at/after `offset`), giving
     `exprStart`; everything before that is the prefix.
  3. Compare the prefix against the last evaluated prefix (cached hash). If it
     **changed**, reset the interpreter namespace and re-execute the entire
     prefix to rebuild state. If unchanged, keep the current interpreter state.
  4. Evaluate the current expression in that namespace and return
     `repr(result)`. Statements with no value (assignments, defs) return empty.
- State carried between calls: the `__main__` namespace + the hash of the last
  evaluated prefix.
- Dependency budget: Go stdlib + cgo binding to libpython only.

### RPC (`internal/rpc`)

Transport: newline-delimited JSON over **stdin/stdout** (the plugin owns the
process via a Neovim job; stdio avoids ports, sockets, and auth). `stderr` is
used for logging.

```go
// protocol.go
type Request struct {
    ID     string         `json:"id"`
    Method string         `json:"method"`     // "eval" | "shutdown"
    Params map[string]any `json:"params"`
}

type Response struct {
    ID    int    `json:"id"`
    Value string `json:"value"`       // repr of result, empty if no value
    Error string `json:"error"`       // error message if Error is non-empty
}
```

`server.go` runs the read→dispatch→write loop:
1. Read a line, decode `Request`.
2. On `eval`, call `lang.Active.Eval(req.Source, req.Offset)`.
3. Encode `Response`, write line, flush.
4. On `shutdown`, close interpreter and exit.

`main.go` calls `lang.Active.Start(ctx)` once, then `rpc.Serve(os.Stdin, os.Stdout)`.

## Neovim Plugin (Lua)

### config.lua

Defaults, merged with user opts in `setup`:

```lua
{
  keymap = "<A-w>",
  bin_dir = nil,   -- resolved to <plugin_root>/bin if nil
  timeout_ms = 5000,
}
```

The server binary for a buffer is **always** `bin_dir/server-<filetype>`
(e.g. `bin/server-python`). No per-language configuration table.

### manager.lua — per-buffer interpreter lifecycle

- Keeps a table `bufnr -> client`.
- On first eval in a buffer, resolves the binary as `bin_dir/server-<filetype>`,
  spawns it via `client.lua`, and caches it. **One process per file.**
- Closes the client on `BufUnload`/`BufDelete` (autocmd) and on `VimLeavePre`.

### client.lua — RPC over job stdio

- `vim.fn.jobstart(cmd, { on_stdout, on_stderr, ... })`.
- Sends requests with `vim.fn.chansend`, tracks pending requests by `id`,
  resolves via callback when the matching `Response` line arrives.
- Buffers partial lines from `on_stdout`.

### parser.lua — offset detection

- Sends the **full buffer text** as `source`.
- Computes `offset` = byte offset of the start of the current cursor line.
- The server (using the real language AST) decides the exact expression bounds;
  the Lua side does not attempt language-aware parsing.

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
      bufnr  = current buffer
      source = full buffer text
      offset = byte offset of start of current line
      client = manager.get_or_spawn(bufnr)
      client:eval(source, offset, function(resp)
          if resp.ok and resp.value ~= "" then
              ui.preview(bufnr, cursor_line, resp.value)
          else
              ui.error(resp.error)
          end
      end)
```

## Data Flow Summary

1. User presses `<A-w>` on a line.
2. Plugin sends the full buffer text + the byte offset of the current line.
3. Manager ensures one server process exists for the buffer.
4. Server parses the source with the native interpreter AST, locates the
   current expression at the offset, and isolates the prefix before it.
5. If the prefix changed since the last call, the interpreter is reset and the
   prefix re-evaluated; otherwise the existing interpreter state is kept.
6. The current expression is evaluated and its `repr` returned.
7. Plugin shows result as inline virtual text.
8. Second `<A-w>` commits the text into the buffer.

## Dependencies

- **Go:** standard library + cgo binding to the language's own interpreter
  library (libpython for Python). No third-party Go modules.
- **Lua:** Neovim built-in APIs only (`jobstart`, extmarks, autocmds). No
  external Lua libraries.
