# Setup

This page takes a person from nothing to `gh board context` answering on their board. The commands are the same on macOS, Linux and Windows; the few per-OS differences are noted where they apply.

## 1. Prerequisites

- GitHub CLI installed and signed in: `gh auth login`. The kit uses the token `gh` already holds, through go-gh; it stores no secret and prints none.
- Token scopes `repo` and `project`. Check with `gh auth status`; add the missing scope with `gh auth refresh -s project`.
- Access on GitHub: the project role `writer` or `admin` for writes, and write permission on the repository where the team creates issues. A person with read access can run every read verb.
- A shell that sets `GH_TOKEN` or `GITHUB_TOKEN` makes `gh` use that token instead of the keyring one (`GH_TOKEN` wins over `GITHUB_TOKEN`). The kit reports the token source in `gh board doctor` and before every write, because writes are attributed to that identity.

## 2. Install the extension

```sh
gh extension install cristianargotti/gh-board
gh board version
```

`gh extension install` downloads the precompiled binary for your operating system and architecture (darwin, linux and windows on amd64 and arm64). Releases are signed with Sigstore and carry GitHub artifact attestations, checksums and an SBOM; `gh board doctor` compares the installed binary with the release checksum, so a tampered binary is visible. Upgrade with `gh extension upgrade gh-board`.

## 3. Point the kit at a board

The zero-friction way is to record the board once:

```sh
gh board use acme/7
gh board status
```

`use` confirms the project exists, writes it to `<config dir>/gh-board/default.yml` and from then on every command reads it, so no command needs `--project`. `gh board use` prints the current default and `gh board use --clear` forgets it.

Any of these also works, in this order of precedence:

1. `--project owner/number` on a command, which overrides any file: `gh board --project acme/7 status`.
2. `--config path` or the environment variable `GH_BOARD_CONFIG`, naming a `board.yml` explicitly.
3. A `board.yml` in the working directory or any parent up to the git root, usually committed in the team repository so that everybody shares it.
4. A per-user file `<config dir>/gh-board/<owner>-<number>.yml` for the project of `--project`, for a board without a repository checkout.
5. The default recorded by `gh board use`, with its per-user file when one is named after it.
6. A single per-user file, when exactly one exists; with two or more and no default the kit asks for `--project` and names them.

Without any file the kit runs in generic mode: every read verb works with the real field names, no role (sprint, epic, lane) is assumed, `move` accepts any option of the field named exactly `Status`, and the capabilities that need a role report "unavailable: not mapped in board.yml". Start from `templates/board.yml`, or let `gh board init` write a draft from discovery, and follow [customizing.md](customizing.md).

Then run the check:

```sh
gh board doctor
```

`doctor` prints the identity and token source, the configuration file in use with its SHA-256, every name in `board.yml` that the board lacks with the commands that depend on it, and the state of the agent guards.

## 4. Install the agents

```sh
gh board agent install --agent all
```

Pick one agent with `--agent claude`, `--agent cursor` or `--agent codex`. The installer is idempotent, prints a diff before writing and `gh board agent uninstall` removes exactly what it added. What each agent receives:

| Agent | Skill | Guard |
| --- | --- | --- |
| Claude Code | the plugin carries the skill, the `PreToolUse` hook on `Bash` and `PowerShell` that runs `gh board guard check --agent claude`, and the mod (`/board` pane, status line, toasts, `tool.call` guard on both tools) | the deny rules written into `permissions.deny` of the user settings, one `Bash(...)` and one `PowerShell(...)` rule per pattern, which Claude Code evaluates before hooks and mods |
| Cursor | `~/.cursor/skills/gh-board/SKILL.md`; with `--scope project`, `.cursor/rules/gh-board.mdc` pointing at it | a `beforeShellExecution` entry in `~/.cursor/hooks.json` running `gh board guard check --agent cursor` with `failClosed: true` |
| Codex | `~/.codex/skills/gh-board/SKILL.md`; with `--scope project`, a marked block in `AGENTS.md` | a `PreToolUse` hook in `~/.codex/hooks.json` running `gh board guard check --agent codex`, plus `~/.codex/rules/gh-board.rules` with forbidden prefix rules, because hook failures do not block in Codex |

Agent-specific steps:

- Claude Code: run `claude plugin marketplace add cristianargotti/gh-board`, then `claude plugin install gh-board@gh-board`. The marketplace and plugin are both named `gh-board`. This loads the skill, hook and mod; `gh board agent install --agent claude` installs the user guard artifacts. Restart Claude Code after installation.
- Codex: trust the hook once through `/hooks` inside Codex; a hook that is not trusted does not run.
- Cursor: nothing else; the hook is read from `~/.cursor/hooks.json` on the next shell command.

Options:

- `--scope user|project`: `user` (default) installs under the home directory only; `project` adds the per-repository files named above.
- `--strict`: adds the strict denies (shell wrappers, absolute paths to `gh`, `env` and `xargs` wrappers, `gh api graphql` with `--input` or `@file` bodies). Off by default because it interferes with ordinary work; see [security.md](security.md).
- `--mcp`: also registers `gh board mcp` as an MCP server of the agent (Claude Code `~/.claude.json`, Cursor `~/.cursor/mcp.json`, Codex `~/.codex/config.toml`), so the agent gets the verbs as typed tools next to the skill; see [mcp.md](mcp.md).

Verify with `gh board doctor`: it checks that the rules and hooks are installed and unchanged (by hash) and that every installed hook names the binary that is running, so a hook left by another installation is reported. Use the offline acceptance test in [releasing.md](releasing.md) to verify denial without sending a destructive command to GitHub.

On Windows the hooks call the binary through an encoded PowerShell invocation (`powershell.exe -NoProfile -NonInteractive -EncodedCommand ...`), which parses the same way in Git Bash, cmd and PowerShell although the extension path holds spaces; elsewhere they call the quoted absolute path. Claude Code on Windows runs commands through its `PowerShell` tool when Git Bash is absent, and the hook, the deny rules and the mod cover that tool as well as `Bash`.

## 5. First context

```sh
gh board context
```

The output is the snapshot the agent starts every session with: the team rules (flow, WIP, SLA, rituals), the sprint and its days left, your open items, the attention list, the triage queue, the epics progress and the deliveries of the week, every section with a minimum share of the budget (about 1,500 tokens by default, `--budget N` changes it) and the rest going to your items; [commands.md](commands.md) has the shares. It always ends with `generated_at`, `completeness` (`complete` or `partial`) and the `omitted` count per section. `--for login` builds it for another person; `--json` returns the same fields structured, which is what the agents and the Claude Code mod read.

From there:

```sh
gh board me
gh board status
gh board attention
```

In the agent, the skill makes it start with `gh board context` and read `completeness` before answering.

## 6. Alerts and the weekly digest (optional)

```sh
gh board watch --once
gh board watch --install --project owner/number --interval 10m
```

`watch --once` evaluates the alert rules of `board.yml`, compares with the last state, notifies only the new or escalated alerts and writes `alerts.json`. `notified.json` records send times to enforce the hourly notification budget across runs. `watch --install` requires a project, supplied by `--project owner/number` or resolved from configuration, and registers the native scheduler as the user: a launchd agent on macOS, a systemd user timer on Linux (a cron line when systemd is absent), a Task Scheduler task on Windows. The Windows task is registered without administrator rights and without a stored password: it runs in the person's session (the only form in which `gh` reaches the keyring and the toast reaches the desktop) and through `conhost.exe --headless`, so no console window opens at every interval. Notifications use `osascript`, `notify-send` or a PowerShell toast, with the text passed as arguments; stdout when no notifier exists. One poller per machine. The Claude Code mod reads `alerts.json` and never polls GitHub itself.

`gh board digest print` prints the week in PT-BR; `gh board digest post` writes a plan that a person applies to publish the status update on the project.

## 7. Remove

```sh
gh board watch --uninstall --project owner/number
gh board agent uninstall --agent all
gh extension remove gh-board
```

Local files live in the per-OS user directories that go-gh resolves: configuration (per-user `board.yml` copies), state (`plans/<id>.json`, `journal/<id>.jsonl`, `audit.jsonl`, `alerts.json`, `notified.json` and the watch lock) and cache (the board schema only, never issue content). `GH_BOARD_HOME` roots the three under one directory, which tests and CI use. Audit and journal entries are pruned after ninety days.
