# gh board

`gh board` is a GitHub CLI extension that lets a team run a GitHub Projects v2 board with AI coding agents (Claude Code, Cursor, OpenAI Codex CLI). The agent reads the board, answers "where are we", creates and moves work, comments and runs the routine steps of the team's process, always as the person who is signed in to `gh`. The board starts from a template and is customized through one file, `board.yml`. Alerts and the weekly digest run without a server, a bot or a shared token. One binary serves people in a terminal, the scheduler and the three agents, by the command line or by MCP.

## Install

```sh
gh extension install cristianargotti/gh-board
gh board use owner/number
gh board agent install --agent all
gh board context
```

For Claude Code, also install the marketplace plugin, which carries the skill, the guard hook and the `/board` pane:

```sh
claude plugin marketplace add cristianargotti/gh-board
claude plugin install gh-board@gh-board
```

The extension installs the binary; `use` records the board so that no command needs `--project`; `agent install` configures the selected agents (`--agent claude`, `--agent cursor` or `--agent codex` installs one, `--mcp` also registers the MCP server); `context` prints the snapshot the agent starts every session with. Releases are signed with Sigstore and carry GitHub artifact attestations, checksums and an SBOM, and `gh board doctor` compares the installed binary with the release checksum. Prerequisites and platform notes are in [docs/setup.md](docs/setup.md).

## Security in one paragraph

GitHub has no permission that distinguishes moving an item from deleting it, so the kit does not claim that an agent holding a developer token cannot destroy anything. It claims, and tests, five things: the binary itself has no code path that deletes, archives, hides or reconfigures anything (a closed mutation allowlist and a source scan test enforce it); consequential changes such as closing an issue, moving to a done status or posting the digest never execute directly, they become a plan that a human applies in a terminal, and the agents are denied `apply`; the known raw destructive commands (`gh project delete`, `gh api -X DELETE`, the forbidden GraphQL mutations and more) are denied inside the three agents; what the kit cannot stop is written down in [docs/security.md](docs/security.md); and reversible operations stay reversible, with before and after values journaled. Every write uses the person's own `gh auth` token, is attributed to them on GitHub and recorded in a local audit log. Strict mode, which also denies shell wrappers and absolute paths, is opt-in.

## The agents kit

One skill ([agents/SKILL.md](agents/SKILL.md)) teaches the three agents the verbs by tier, the `context` snapshot and the rules. `gh board agent install` writes it next to each agent's deny rules and a PreToolUse hook that calls `gh board guard check`, idempotently and after printing a diff; `agent uninstall` removes exactly what was installed. Claude Code gets the marketplace plugin above, with a mod that shows the alerts `gh board watch` writes in the `/board` pane. `gh board doctor` reports every installed artifact and hook by hash and the binary each hook names. The verified hook and rule contracts are in [docs/agent-contracts.md](docs/agent-contracts.md); the offline acceptance that proves the guards without sending a destructive command to GitHub is in [docs/releasing.md](docs/releasing.md).

## MCP

`gh board mcp` serves the kit as a Model Context Protocol server over stdio, inside the same binary and with the same rules: the read verbs, the direct writes and the plan verbs `close`, `tidy` and `digest_post` become tools whose catalog is introspected from the command tree, so the help and the tools cannot drift apart. A plan tool returns the plan id and the exact `gh board apply <id>` line; `apply` is never a tool. `gh board agent install --mcp` registers the server in Claude Code, Cursor and Codex, and `doctor` checks the entries. Details, result shapes and limits are in [docs/mcp.md](docs/mcp.md).

## Documentation

| Page | Content |
| --- | --- |
| [docs/setup.md](docs/setup.md) | Install the extension, point it at a board, install the agents, first `context`. |
| [docs/commands.md](docs/commands.md) | The command catalog by tier, flags, references and exit codes. |
| [docs/customizing.md](docs/customizing.md) | `board.yml` key by key, resolution order, what `init` infers, structure and views. |
| [docs/security.md](docs/security.md) | Assurance levels, allowlist, guards and their limits, strict mode, residual risks. |
| [docs/mcp.md](docs/mcp.md) | The MCP transport: `gh board mcp` and `agent install --mcp`, the tool catalog and its limits. |
| [docs/agent-contracts.md](docs/agent-contracts.md) | The Codex and Claude Code hook contracts, rule checks and the skill source. |
| [docs/onboarding.md](docs/onboarding.md) | Team onboarding checklist (PT-BR) with the measured time of each step. |
| [docs/faq.md](docs/faq.md) | Frequently asked questions. |
| [docs/releasing.md](docs/releasing.md) | Release checks, including offline guard acceptance. |
| [templates/](templates/) | The reference `board.yml` (an example board, `acme/7`), the issue forms and the template README (PT-BR). |

Board content shown to the team (template, forms, alerts, digest, plan descriptions) is in PT-BR; everything else is in English.

## Status

Releases are built by the release workflow from tags, signed and attested; binaries exist for macOS, Linux and Windows on amd64 and arm64. Changes are listed in [CHANGELOG.md](CHANGELOG.md), the repository laws in [CONTRIBUTING.md](CONTRIBUTING.md), and the license is MIT ([LICENSE](LICENSE)).
