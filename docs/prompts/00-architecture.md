Create a detailed plan for a neovim plugin `revaluator` which will:

- Accept any opened input file. This could be any language, but for now ONLY Python will be implemented.
- Accept a configurable keybind, defaulting to Alt-W. This will show a suggestion similar to other AI tab completion plugins, however this plugin is deterministic.
- When the keybind is pressed:
  - Parse the current line as an expression in the language of the file
  - Spawn an interpreter for the matching language. This interpreter will be spawned only once per file. Do not spawn it multiple times to interpret successive expressions.
  - Parse and evaluate the expression in the interpreter. Expressions may span multiple lines, so assume that the expression starts at the current line. If this is the first time the interpreter is spawned, then evaluate every line BEFORE the current line; only after you evaluated everything before will you run the expression.
  - Show the suggestion at the end of the current line. It will not be written to the file, and it will only be shown visually.
  - If the user presses Alt-W again, then write the suggestion into the end of the current line.

This repository will contain:
- The neovim plugin, written in Lua
- A Go server which the plugin will communicate with. The Go server will maintain the interpreter and evaluate the expressions, returning the result.
  - Add a Makefile with rules for each language. for now, only the `python` rule is present. Each language will correspond to a separate binary, i.e. `server-python`, `server-javascript`, ...
  - Add one internal module with files for multiple languages. Use build flags to activate that specific language for the build.

Rules:
- Organize the code for maintainability.
- Keep the amount of dependencies small.
- There are no other files in the directory. You will read only this file.

Output:
You will output into `docs/prompts/00-architecture.out.md`. Write a single file, and keep it concise with no filler.
