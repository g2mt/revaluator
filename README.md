# revaluator

Evaluate the expression under the cursor using a persistent language interpreter. Preview the result as inline virtual text; press the key again to commit it into the buffer.

## How it works

- Neovim plugin (Lua) talks **JSON-RPC over stdio** to a per-buffer Go server.
- The Go server embeds the language's own interpreter (e.g. **libpython** via cgo).
- Full buffer text is sent on each eval. The server resets the interpreter and re-evaluates the prefix only when it changes, keeping interpreter state across calls.

## Requirements

- Neovim
- Go (for building the server)
- Language-specific requirements:
  - Python: libpython3-dev

## Quick start

In your Neovim config:

```lua
require("revaluator").setup()
```

Default keymap:

  - `<A-w>`: press to preview, press again to commit.

## License

MIT License. Contains AI generated code.
