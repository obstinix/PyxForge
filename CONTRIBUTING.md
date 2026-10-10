# Contributing to PyxForge

## Branches

`main` is the only long-lived branch. The maintainer commits to it directly in small, verified
increments. Contributors open a pull request against `main`.

## Before you commit

Enable the shared pre-commit hook once per clone:

```sh
git config core.hooksPath .githooks
```

It blocks forbidden web technology, unformatted Go, local environment files and, when
[gitleaks](https://github.com/gitleaks/gitleaks) is installed, anything that looks like a secret.

Then run the same checks CI runs:

```sh
go run ./tools/forbidcheck
gofmt -l .
go vet ./...
staticcheck ./...
go test -race ./...
(cd nvim && stylua --check . && luacheck .)   # when you change the Lua configuration
```

The integration tests run real Neovim, Git, QEMU, `qemu-img`, GDB and NASM when they are
installed; `go test -v` shows which ones skipped on your machine. [`docs/INSTALL.md`](docs/INSTALL.md)
lists how to install them.

If you touch `legacy/core`, also run `cargo fmt --check`, `cargo clippy --all-targets -- -D warnings`
and `cargo test` there.

## Rules that are not negotiable

- **No web technology in the application.** No HTML, CSS, JavaScript, TypeScript, npm, Electron,
  Tauri, WebView, CodeMirror or Monaco outside `docs/` and `legacy/`. `tools/forbidcheck` enforces
  this in CI.
- **The editor is real Neovim.** Do not reimplement Vim behaviour.
- **Offline first.** No network request at startup; networked features are optional and off
  until the user enables them.
- **No fake data.** A panel whose tool is not built says so; nothing invents values.
- **Secrets never enter the repository.** No tokens, keys or `.env` files. A committed secret is
  treated as compromised and rotated.
- **Design values come from the design system.** Read
  [`docs/design/DESIGN_SYSTEM.md`](docs/design/DESIGN_SYSTEM.md) before changing UI.
- **Every action is a command** in `internal/command` before it gets a keybinding, and shell
  keybindings are Ctrl+Shift chords only ([`docs/architecture/KEYMAP.md`](docs/architecture/KEYMAP.md)).

## Commit messages

One meaningful, completed change per commit, in Conventional Commits form:

```text
<type>(<area>): <what changed>
```

Types: `feat`, `fix`, `docs`, `build`, `ci`, `test`, `chore`. Areas in use include `editor`,
`terminal`, `build`, `problems`, `git`, `qemu`, `debug`, `inspect`, `snapshot`, `proc`,
`security`, `workspace`, `keys`, `cli`, `doctor`, `release`, `ui`, `theme`, and `core` for the
legacy Rust core. The body explains why, and how the change was verified.

## Tests

Tests must be able to fail. UI behaviour is tested with Fyne's test driver; parsers and services
with table tests and golden cases ported from the 2.x Rust tests.
