# ADR 0007: Agents

- **Status:** Accepted (design; nothing is built yet, #73)
- **Date:** 2026-11-07
- **Covers:** feature parity rows 23, N10 and N11; roadmap Phases 6 and 7

## Context

The plan asks for agents that are not tied to one provider and that work in isolated
worktrees. A person approves every change before it reaches the project. The flagship is a
debug loop: build, boot in QEMU, take a GDB snapshot at the fault, hand it to an agent, and
apply the patch the person approves. None of this exists yet. Before any code, this record
settles four things the issue asks for: the provider boundary, how far isolation goes, how a
change gets from an agent into the project, and what leaves the machine.

PyxForge's existing commitments limit the design. It makes no network request at startup,
needs no account and sends no telemetry (README). Opening a project runs nothing from it, and
running a project's own commands needs trust (D14). Every process PyxForge starts ends with it
(D10). Machine records are versioned JSON outside the project (D11). The worktree model comes
from Orca (`REFERENCE_PROJECTS.md`): one agent, one worktree, and diff review as the only path
into the user's branch.

## Options

1. **Run agent programs the user already has.** Claude Code, Codex CLI, OpenCode and similar
   tools run in a terminal and edit files in a folder. PyxForge starts one per task, in a
   worktree of its own, and reviews what it leaves behind. Each tool keeps its own login, model
   choice and permission prompts.
2. **Build an agent into PyxForge.** PyxForge calls a model's API itself and runs the tools
   (read, edit, build) for it. That means storing keys, choosing models and building a tool
   loop and its permissions. The result would rival programs that already do this well.
3. **Editor actions only.** "Explain this", "suggest a fix" through a model endpoint, with no
   worktrees. This is too small for the debug loop.

## Decision

Option 1 for agents. A narrow slice of option 2, a local model endpoint, serves the editor
actions in row 23.

### Providers

`internal/agents` defines the boundary. Nothing else in PyxForge names a provider.

```go
type Provider interface {
	ID() string                                  // "claude-code", "codex", "opencode", "local"
	Detect(context.Context) Availability         // found and its version, or why not and how to get it
	Start(context.Context, Run) (Session, error) // a Run is the task, its worktree and its context
}

type Session interface {
	Events() <-chan Event               // output and state: working, needs input, finished, failed
	Send(context.Context, string) error // a follow-up, such as a review comment
	Stop(context.Context) error         // ends the agent's whole process tree
}
```

- **Terminal agents** are one adapter with a table of providers: the program, how the task is
  passed (an argument, stdin, or typed at its prompt), and how a finished state is detected
  (its exit, or its own completion hook where it has one). The program runs in a terminal
  session like the Terminal panel's (D13), so the user sees it and answers its questions there.
- **The local endpoint** speaks the OpenAI-compatible chat API that Ollama, llama.cpp's server
  and LM Studio serve. By default it accepts only a loopback address (`127.0.0.1`, `::1`,
  `localhost`). It has no tools of its own: it answers in text, or proposes a unified diff that
  goes through the same review as an agent's.
- **Detection is passive.** PyxForge looks for each program on `PATH` or at a path the user set,
  only when Agents opens or the user asks. It never downloads or installs an agent. A missing
  program disables that provider and says how to install it. It never blocks startup or any
  other provider.
- **A fake provider** (a test program) drives the adapter in tests: it reads the task, edits
  files, prints output and exits with a chosen code. Row 23 asks for exactly this test.

### Isolation: a worktree per run

- Each run gets a Git worktree on a new branch, `pyxforge/agent/<run>`, made from the commit the
  user is on. The user can choose to carry over their uncommitted changes to tracked files:
  PyxForge makes a stash commit with `git stash create`, which leaves their tree alone, and
  applies it in the worktree. New files the user hasn't added stay behind.
- Worktrees live in PyxForge's cache folder (`<cache>/PyxForge/worktrees/<repo hash>/<run>`),
  not in the project. The explorer, search and builds never see them.
- PyxForge keeps a record of the worktrees it made. It removes only those, and it retries a
  removal that failed (a Windows file still open). This is Orca's two-level record: Git's
  porcelain list plus PyxForge's metadata. `internal/git` already lists, adds, removes and prunes
  worktrees.
- A folder that isn't a Git repository can't host agents. Agents offers to create the
  repository, as the Git tab does.
- **A worktree isolates files, not the agent.** The agent program runs as the user, with the
  user's rights. It can read any file the user can, run any program and use the network. The
  worktree keeps its edits off the user's branch until they are reviewed. It doesn't contain a
  program that misbehaves. PyxForge never passes the flags that switch off an agent's own
  permission prompts (such as "skip permissions" or full-auto modes). If the user wants those,
  they set them in that provider's own configuration. Sandboxing at the operating-system level
  (AppContainer on Windows, Landlock or bubblewrap on Linux) is future work, not part of this
  design.

### From an agent to the project: review, then apply

- **Agents never touch the user's branch.** An agent may commit in its worktree; that branch is
  its own.
- **The review compares the worktree with what it started from.** That is the commit, plus the
  user's carried-over changes if there were any (the stash commit's tree), so applying the
  review never repeats edits the user's tree already has. The comparison includes the agent's
  commits, its uncommitted edits and its new files. PyxForge builds the diff with a
  temporary index (`GIT_INDEX_FILE`), so the worktree's own index is left alone. PyxForge's own
  files in the worktree (`.pyxforge-agent/`) are left out.
- **The review shows files and hunks.** The user takes all of it, some files or some hunks.
  They can comment on a line, and the comment goes back to the agent as a follow-up (Orca's diff
  comments).
- **Changes that alter what runs are flagged.** These are build configuration
  (`pyxforge.toml`), build scripts and makefiles, `.gitattributes` (filters), Git hook folders,
  CI workflows and editor task files. Applying them changes what Build, Run or Git will execute.
  Trust doesn't ask again for a folder that is already trusted, so the review says so before the
  user applies.
- **Apply puts the change in the user's working tree, uncommitted.** It never commits. PyxForge
  uses `git apply`, which applies the whole patch or nothing. If the user's files changed since
  the run started and the patch no longer fits, PyxForge names the files that conflict and keeps
  the review open. The user commits from the Git tab as usual.
- **Discard removes the worktree and its branch.** Kept runs stay listed until discarded.

### Trust

- **Agents run only in trusted folders (D14).** An agent reads the project and usually builds
  it, so in an untrusted folder Start asks for trust first, as Build does.
- **Every run is started by the user.** Nothing starts at launch or in the background. An agent
  never starts another run or applies a review. The debug loop below asks before each round.
- **The agent's environment is PyxForge's own, minus PyxForge's variables.** It loses
  `PYXFORGE_*` and the askpass variables PyxForge gives Git. A run therefore can't reach PyxForge's
  credential prompt and pass for PyxForge.
- **PyxForge stores no keys.** Terminal agents keep their own sign-in. The local endpoint needs
  none.
- **Runs end with PyxForge (D10)**, and Stop ends the agent's whole process tree.

### What leaves the machine

This is the part a user must be able to read before the first run, so the Start dialog shows
it every time, with the provider's name:

- **The provider receives the task the user typed and the context the user attached.** That can
  be a selection, a file, the build's diagnostics or a context bundle. Each item is listed with
  its size, and the user can open it before sending.
- **A terminal agent reads the project's files itself.** It can read every file in its worktree,
  and the files it reads go to its provider under that provider's terms. The dialog says this
  in so many words. To keep a file out, leave it out of the commit the run starts from.
- **PyxForge itself sends nothing else.** It sends no other project, no environment variables,
  no telemetry and no usage counts. The local endpoint sends only to the address the user
  configured, and by default only to a loopback address.
- **Transcripts stay on the machine.** Each run's output is kept in the cache folder with the
  run, and goes when the run is discarded. Settings has Delete All Agent Data.
- **The context bundle uses paths relative to the project**, so the user's home folder and user
  name aren't in it.

### The debug loop and its context bundle (N11)

```text
build fails, or the machine stops at a fault
  → context bundle (versioned JSON)
  → agent run in a worktree → review → apply
  → build → boot in QEMU → the next stop, and the next bundle
```

- **The bundle** is `pyxforge.agent-context`, version 1, written to `.pyxforge-agent/context.json`
  in the run's worktree, where the agent can read it. It holds:
  - the task;
  - the base commit and the build profile;
  - the build command (as arguments), its exit code, its diagnostics (file, line, column,
    severity, message) and the end of its log;
  - for a machine stop: the QEMU arguments, the reason for the stop, the CPU mode, the program
    counter, the registers, the backtrace, memory near the program counter and the stack, and
    the disassembly there;
  - the end of the serial output.

  The machine part is the diagnostic snapshot PyxForge already records (D11, `internal/snapshot`).
- **The schema is tested.** It is a JSON Schema in `docs/schemas/`, with golden tests for a
  build failure and for a fault. The bundle has a size cap (256 KiB), applied by trimming logs
  and memory, never the diagnostics.
- **What triggers it.** "Ask an Agent to Fix This" appears on a failed build and on a stopped
  machine. To catch a triple fault, a debug boot can stop instead of resetting
  (`-no-reboot -no-shutdown`). PyxForge then snapshots the paused machine.
- **Each round needs the user.** After an apply, the user chooses Apply, or Apply and Run (build
  and boot again). The loop never applies, rebuilds or starts another round on its own. Each
  round's bundle is kept with the run, so the rounds can be compared.

## Not part of this design

- A model built into PyxForge, keys stored by PyxForge, or a choice of models made by PyxForge.
- Agents in the cloud, accounts, usage tracking or telemetry.
- Agents that commit to, push or merge into the user's branches.
- Starting several agents on one task and comparing them, Orca's fan-out. The design allows it
  (one run, one worktree), and it can come later.
- Letting an agent drive PyxForge, for example a protocol through which it sets breakpoints or
  reads registers. That gives a program control of the debugger, so it needs its own trust
  decision and its own record.
- Sandboxing the agent at the operating-system level.

## Consequences

- PyxForge's agent support is only as good as the programs the user installs. In exchange,
  PyxForge holds no keys, no model choice and no tool loop, and it gains each program's
  improvements for nothing.
- The worktree and the review are PyxForge's to get right. They are also the parts that need
  no network and can be tested completely with the fake provider.
- Without Git, there are no agents.
- The work is split into issues: providers and the fake provider (#81), agent worktrees (#82),
  the terminal adapter (#83), review and apply (#84), the Agents tab and Start dialog (#85), the
  context bundle (#86), the debug loop and its recorded run with a faulty bootloader, the
  Phase 7 gate (#87), and the local endpoint with the editor actions (#88).
