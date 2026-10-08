# MCP transport

`gh board mcp` serves the kit to an agent as a Model Context Protocol server over standard input and output. It is another door to the same core with the same rules (ADR-005): every tool runs the verb of the same name in process, with the same preconditions, exit codes, journal and audit line as the command line. Nothing is reachable through MCP that the command line refuses, and `apply` is never a tool.

## When to use it

The skill and the command line remain the reference. The MCP server is for the moments when an agent works better with typed tools than with shell calls: the arguments are validated against a schema before the verb runs, the result arrives as structured JSON, and the agent does not spend turns reading `--help`. Claude Code, Cursor and Codex load MCP tools on demand, so the catalog costs little until a tool is used; each tool definition costs about 150 to 300 tokens when loaded, and `context` returns the same budgeted snapshot the command prints.

## Tools

The catalog is derived from the command tree: the description is the verb's help, the flags become typed fields with their defaults and descriptions, and the positional arguments are declared by each verb. `gh board mcp` and `gh board <verb> --help` are one catalog.

| Tier         | Tools                                                                                                                   | Fields every tool of the tier takes      |
| ------------ | ----------------------------------------------------------------------------------------------------------------------- | ---------------------------------------- |
| Read         | `context`, `status`, `attention`, `roadmap`, `item`, `list`, `search`, `epics`, `sprint` (`which`: `current` or `next`) | `project`                                |
| Direct write | `move`, `assign`, `unassign`, `set`, `comment`, `new`                                                                   | `project`, `dry_run`, `reason`, `expect` |
| Plan         | `close`, `tidy`, `digest_post`                                                                                          | `project`, `dry_run`, `reason`, `expect` |

`project` overrides the project for one call. The `--project` and `--config` flags of `gh board mcp` itself apply to every call.

Never tools: `apply`, `agent`, `guard`, `watch`, `init`, `milestone create` and `retarget`, `plan list` and `show`, `schema`, `doctor`, `log`, `me`, `standup`, `digest print`, `version`. A client that calls one of them gets the JSON-RPC error `Unknown tool`.

## Results

Every call answers with a short text line, the `--json` document of the verb as a second text block, and the same document inside `structuredContent` under `data`, wrapped in an envelope with `tool`, `exit_code` and `status`:

- exit code 0: `isError` is false; `data` holds the document.
- exit code 5 with a plan (`close`, `tidy`, `digest_post`, `move` to a done status, a bulk above the team threshold): `isError` is false; the envelope adds `plan_id`, `dry_run` and, unless it was a dry run, `apply`, the exact `gh board apply <id>` line the person runs in a terminal. The text line repeats it.
- any other exit code (1 usage, 2 not found, 3 policy, 4 drift, 7 API): `isError` is true; the envelope carries `message` and the code, and the text line reads `<tool>: <message> (exit code N, <status>)`. A transport error never stands in for a refused write.
- arguments the schema rejects (unknown field, wrong type, missing positional) are tool errors too, with `status` `invalid-arguments`, so the model can correct the call.

A `comment` or `new` body of `-` reads an empty input under MCP, because standard input carries the protocol; pass the text itself or `@path`.

## Install

```sh
gh board agent install --agent all --mcp
```

`--mcp` adds the server entry to the selected agents, next to the skill and the guards; the installer stays idempotent, prints the diff first and records the entry so that `gh board agent uninstall --mcp` removes exactly that entry and `gh board agent uninstall` removes it with the rest. The entry runs the absolute path of the installed binary with the argument `mcp`.

| Agent       | User scope                                                                                                         | Project scope                                            |
| ----------- | ------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------- |
| Claude Code | `mcpServers.gh-board` in `~/.claude.json`                                                                          | `mcpServers.gh-board` in `.mcp.json` at the project root |
| Cursor      | `mcpServers.gh-board` in `~/.cursor/mcp.json`                                                                      | `mcpServers.gh-board` in `.cursor/mcp.json`              |
| Codex       | `[mcp_servers.gh-board]` in `~/.codex/config.toml`, inside a marked block (`# gh-board:begin` to `# gh-board:end`) | same file; Codex has no project servers                  |

A `[mcp_servers.gh-board]` table that already exists outside the block (written by `codex mcp add`) is left alone and reported, because a second definition would break the whole file for Codex. Restart the agent after installing; Claude Code also accepts the entry through `claude mcp add --transport stdio --scope user gh-board -- /path/to/gh-board mcp`.

`gh board doctor` lists every recorded entry under "MCP servers" with the binary it names and fails the row when the file lost the entry or names another binary, the same check the hooks get.

## Protocol

Stdio, one JSON-RPC 2.0 message per line, standard library only. The server speaks the current revision (2026-07-28: `server/discover`, the protocol version in `_meta` of every request, `resultType` in every result) and the handshake revisions 2025-11-25, 2025-06-18, 2025-03-26 and 2024-11-05 (`initialize`, `notifications/initialized`, `ping`). Claude Code probes with `server/discover` and stays modern; a client that opens with `initialize` is served the version it asked for. Unknown methods get `-32601`, a request before `initialize` on a legacy session gets `-32600`, a version the server does not speak gets `-32022` with the supported list, and the server exits when its input closes. It serves one request at a time and starts no goroutines.

## Security

The MCP server adds no mutation and no tier: the mutation allowlist, the plan requirement for consequential changes and the exit codes are those of [security.md](security.md). `apply` is absent from the catalog, so the plan a tool returns still needs a person in a terminal. The agent guards are still needed: the server covers only the calls that go through it, and a raw `gh project` or `gh api` call from the agent's shell is unaffected by it, which is what the deny rules and hooks of `gh board agent install` are for. Issue titles, bodies and comments in the results are data, delimited and truncated as in every other output of the kit.
