# Reference Projects

Studied on 2026-10-08 from shallow clones in `C:\ProjectsPP\pyxforge-study\` (outside the repo).
Study only: no code from any of these enters PyxForge.

| Project | Commit studied | License | Stack | Why it matters |
|---|---|---|---|---|
| Neovim | `27ee55c` (2026-10-07), version 0.13.0-dev | Apache-2.0, with parts under the Vim license | C, Lua | The editor engine (D4) and its UI protocol |
| Orca (`stablyai/orca`) | `10a2a6c` (2026-10-07), app 1.4.214 | MIT | Electron, React, node-pty, `@anthropic-ai/claude-agent-sdk`, PostHog | Parallel agents in isolated worktrees, diff review |
| NvChad | `add44b9` (2026-07-03) | **GPL-3.0** | Lua | Organization of a curated Neovim config |

NvChad is GPL-3.0: it was read for structure and concepts only. Nothing from it, not even
keymap tables, may be copied into this Apache-2.0 repository.

## Neovim

**Architecture.** A single editor process with an msgpack-RPC API. External UIs attach with
`nvim_ui_attach(width, height, options)` and receive `redraw` notifications: batches of update
events, ending with `flush` when the screen is consistent (`runtime/doc/api-ui-events.txt`).
UIs must render only at `flush` and must ignore unknown events and extra parameters
(forward compatibility, `|api-contract|`).

**Editor model and the protocol surface PyxForge needs (D4).**

| Option | What it gives PyxForge | Phase |
|---|---|---|
| `ext_linegrid` | Line-based `grid_line` (runs of cells with highlight ids and repeat counts), `grid_scroll`, `grid_cursor_goto`, `grid_resize`, `grid_clear`, plus `hl_attr_define` and `default_colors_set`. The minimum for the spike. | 3 spike |
| `ext_multigrid` | One grid per window with `win_pos`, `win_float_pos`, `win_viewport` (topline, botline, line count, scroll delta). Lets Fyne own splits and smooth-scroll metrics. | 3 integration |
| `ext_cmdline` | `cmdline_show/pos/hide`: Fyne draws the `:` command line, which lets it share the command palette's look. | 3 |
| `ext_popupmenu` | `popupmenu_show/select/hide`: Fyne draws completion as a native overlay (a Glass surface under D3). | 3 |
| `ext_messages` | `msg_show`, `msg_clear`, `msg_history_show`: messages route into the notification system. | 3 |
| `ext_hlstate` | Semantic highlight info per id, so theme tokens can map highlight groups. Supports the D2 editor/theme sync boundary. | 3 |
| `ext_tabline` | Optional; PyxForge already has workspace tabs. | later |

**Startup.** `nvim --embed` (never with `--headless`) pauses before loading startup files until
the UI calls `nvim_ui_attach`, so the embedder can set `g:` variables first, and can register a
`VimEnter` autocmd that issues a blocking `rpcrequest` back to the UI for post-config setup.
PyxForge will start it as `nvim --embed -u <repo>/nvim/init.lua` with isolated `XDG_CONFIG_HOME`,
`XDG_DATA_HOME`, `XDG_STATE_HOME` and `XDG_CACHE_HOME` (Section 10.4) so the user's own config
is never read or written.

**Extension model.** Lua plugins on `runtimepath`; `vim.lsp` and `vim.treesitter` are built in;
`vim.diagnostic` namespaces let an external process push diagnostics (row 6 of the parity matrix).

**Performance decisions to copy.** Batch then flush; repeat counts in `grid_line` cells;
`grid_scroll` instead of redrawing moved lines. A Fyne renderer should keep a cell buffer per grid,
apply events, and repaint once per `flush`.

**Go client.** `github.com/neovim/go-client` (Apache-2.0) is the official Go msgpack-RPC client
and is the obvious candidate for the Phase 3 spike. It must pass the dependency rule (Section 8)
before it is added.

## Orca

**What it is.** A desktop "agent IDE": run any terminal CLI agent (Claude Code, Codex, OpenCode
and others) side by side, each in its own git worktree, track them in one place, review their
diffs, and merge the result.

**Architecture.** Electron main process (`src/main/`, about 80 service folders: `agent-launch`,
`agent-hooks`, `git`, `pty`, provider clients for GitHub, GitLab, Gitea, Linear), a preload API,
a React renderer, shared types (`src/shared/`), a CLI (`src/cli/`) and a mobile companion. Not
reusable by PyxForge (Electron and React are forbidden); the workflows are what matter.

**Agent model.**
- Provider-agnostic: anything that runs in a terminal is an agent; launch, prompt delivery and
  status hooks are per-provider adapters (`src/main/agent-launch/`, `src/main/agent-hooks/`).
- One agent, one worktree; "fan one prompt across five agents", compare, merge the winner.
- Diff review with line comments sent back to the agent (`DiffComment` in
  `src/shared/worktree/types.ts`).
- Notifications and unread state when an agent finishes or needs attention.

**Workspace and worktree model** (`src/shared/worktree/types.ts`, `ownership.ts`, `removal.ts`).
- Git-level record from `git worktree list --porcelain`: path, head, branch, bare, sparse,
  locked and reason, prunable and reason (Git 2.36+ `prunable` field, with a path-existence probe
  as fallback), main-worktree flag, and a recorded removal error so a failed delete can be retried.
- App-level record enriched on top, with a stable id `${repoId}::${path}`, display name, status,
  and links to issues and pull requests.
- Head and branch freshness read from git metadata files without spawning git, so status churn does
  not trigger a full rescan.
- Explicit ownership rules decide which worktrees the app may remove.

**What PyxForge takes for Phase 6.**
1. Provider interface with per-provider launch and status adapters; a missing CLI disables only
   that provider (Section 13.8).
2. Two-level worktree record (git porcelain plus app metadata), stable ids, prunable handling,
   ownership checks before removal, and retry of a failed removal.
3. Diff review as the only path from agent output to the user's branch: agents never commit.
4. Fan-out of one task to several agents, compared side by side. This fits the agent debug loop
   (Section 13.9): the same crash bundle can go to two providers.

**What PyxForge leaves.** Cloud, mobile and SSH features; account switching and usage tracking;
PostHog telemetry (PyxForge makes no network calls at startup and has no telemetry); the
browser-based "Design Mode".

## NvChad

**Architecture.** A deliberately small core (`lua/nvchad/`): `options.lua`, `mappings.lua`,
`autocmds.lua`, one file per plugin configuration under `configs/`, and a single
`plugins/init.lua` spec for the lazy.nvim plugin manager. The UI layer (statusline, tabline,
dashboard, theme compiler "base46") lives in separate repositories.

**Patterns worth learning.**
- **Lazy loading by trigger.** Plugins load on an event (`InsertEnter` for completion,
  `BufReadPost`/`BufNewFile` for Tree-sitter, a custom `User FilePost` event for LSP, gitsigns and
  indent guides) or on a command (`cmd = "Telescope"`), so startup stays fast.
- **Compiled theme cache.** base46 compiles highlight groups into cached bytecode at install time
  instead of computing them at startup. The PyxForge equivalent: generate the Neovim highlight
  table from Go theme tokens once per theme change (the D2 synchronization boundary) and cache it.
- **One config file per plugin**, matching Section 10.3's `nvim/lua/pyxforge/*.lua` layout.

**What PyxForge must not do.** NvChad installs LSP servers through Mason and builds parsers with
`:TSUpdate`, which download from the network. In PyxForge these happen only through an explicit
setup command, never at startup (Section 10.4, Section 12). Plugins are pinned with a lockfile.

## Cross-cutting conclusions

- **Neovim owns text; PyxForge owns chrome.** Use `ext_cmdline`, `ext_popupmenu` and
  `ext_messages` so the command line, completion and messages share PyxForge's overlay language
  rather than being drawn inside the grid.
- **Every long-lived external tool is a session**, not a per-request process: Neovim RPC, QMP,
  GDB/MI and agent PTYs all follow Orca's and Neovim's model of an owned process with an event
  stream.
- **Startup stays local.** Lazy-load inside Neovim, compile the theme cache once, never download
  at launch.
