# revaluator

Evaluate the expression under the cursor using a persistent language interpreter. Preview the result as inline virtual text; press the key again to commit it into the buffer.

## How it works

- Neovim plugin (Lua) talks **JSON-RPC over stdio** to a per-buffer server binary.
- The server uses the language's own **AST** to find the expression under the cursor and split it from the prefix.
- Prefix statements are evaluated in a persistent runtime so state accumulates across lines.

## Requirements

- Neovim
- Go (for building the Python server)
- Node.js (for the JavaScript server)
- Language-specific requirements:
  - JavaScript: npm
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
