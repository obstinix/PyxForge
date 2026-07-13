# PyxForge

Core tooling for from-scratch OS and systems development.

## Project Status
Early development — core setup in progress (Phase 0).
For details on the project scope and milestones, see [docs/PRD.md](docs/PRD.md).

## Verify it works

To manually verify the setup:
1. Build the Rust core:
   ```bash
   cd core && cargo build
   ```
2. Build the VS Code extension:
   ```bash
   cd ../extension && npm install && npm run build
   ```
3. Open the `extension/` folder in VS Code, press `F5` to launch the Extension Development Host.
4. Open the Command Palette (`Ctrl+Shift+P` or `Cmd+Shift+P`), run `PyxForge: Ping Core`.
5. Expect an info message showing `status: ok`, `message: pong`, and the core's version.
