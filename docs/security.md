# Security model

GitHub has no permission that distinguishes moving an item from deleting it: a token that can write a project can delete its items. The kit therefore does not claim that an agent holding a developer token cannot destroy anything. It claims, and tests, five things, listed below from the strongest to the one that is only a written statement.

## The five assurance levels

| Level | Claim | Enforced by |
| --- | --- | --- |
| 1 | `gh board` itself cannot destroy, hide or reconfigure | absence of code paths, mutation allowlist, source scan test |
| 2 | Consequential changes need a human in a terminal | plan and apply, `apply` denied for agents |
| 3 | Known raw destructive commands are denied in the agents | deny rules and hooks in Claude Code, Cursor and Codex |
| 4 | What the kit cannot stop is written down | the residual risks below |
| 5 | Reversible operations stay reversible | `restore` (unarchive), `reopen`, journaled before and after values |

## Level 1: no destructive path in the binary

Every GraphQL document is an embedded file and the adapter exposes exactly these mutations:

`createIssue`, `updateIssue` (title, body and milestone only; assignees and labels go through their dedicated mutations), `addAssigneesToAssignable`, `removeAssigneesFromAssignable`, `addLabelsToLabelable`, `removeLabelsFromLabelable`, `addComment`, `closeIssue`, `reopenIssue`, `addSubIssue`, `updateIssueIssueType`, `setIssueFieldValue`, `updateIssueFieldValue`, `addProjectV2ItemById`, `updateProjectV2ItemFieldValue`, `unarchiveProjectV2Item`, `createProjectV2StatusUpdate`, `createProjectV2Field` (plans only, new fields only), `createLabel` (plans only), `createMilestone` through REST (plans only), `copyProjectV2` (plans only), `linkProjectV2ToRepository` (plans only).

A unit test reads every embedded document and every Go source on every build and fails when it finds a mutation outside that list, and explicitly when it finds `deleteProjectV2`, `deleteProjectV2Item`, `deleteProjectV2Field`, `deleteProjectV2View`, `deleteProjectV2Workflow`, `deleteProjectV2StatusUpdate`, `archiveProjectV2Item`, `clearProjectV2ItemFieldValue`, `updateProjectV2`, `updateProjectV2Field`, `updateProjectV2Collaborators`, `unlinkProjectV2FromRepository`, `unmarkProjectV2AsTemplate`, `deleteIssue`, `transferIssue`, `removeSubIssue`, `deleteIssueComment`, `updateIssueComment`, `deleteIssueField`, `deleteIssueFieldValue`, `deleteIssueType`, `deleteLabel`, `deleteMilestone`, or any REST call with the method `DELETE`.

Queries use variables only. Every option, field, login, label, milestone and status is validated against the discovered schema before a mutation is sent, with a "did you mean" suggestion on mismatch. Views, workflows, insights, visibility, settings, collaborators, transfers, clearing values, editing existing fields or options, editing or deleting comments and template marks have no code in the binary at all; `createProjectV2Workflow` does not exist in the GitHub API, so workflows reach a new board only through the template copy.

## Level 2: consequential changes need a human

Verbs are classified by what happens downstream, not by the name of the mutation: moving an item to a done status closes the issue through the board's workflow, so it is guarded like `close`.

| Tier | Verbs | Properties |
| --- | --- | --- |
| Read | `context`, `status`, `me`, `item`, `list`, `epics`, `roadmap`, `sprint`, `attention`, `search`, `schema`, `digest` (print), `standup`, `doctor`, `log`, `plan list`, `plan show`, `version` | no side effects |
| Direct write | `move` to a non-done status, `assign`, `unassign`, `set`, `sprint set`, `estimate`, `dates`, `milestone set`, `comment`, `link --parent`, `label`, `new`, `reopen`, `restore` | reversible, one item (bulk above `policy.bulk_threshold` becomes a plan), journaled, `--dry-run` and `--reason` available |
| Plan | `close`, `move` to a done status, `digest post`, `tidy`, `init`, `milestone create`, `milestone retarget`, any bulk write above the threshold | write a plan; nothing changes until `apply` |
| Apply | `apply <plan>` | interactive terminal required, denied for agents by the guards, re-reads preconditions, journals every step |
| Absent | delete or archive anything, remove from project, unlink, views, workflows, insights, visibility, settings, collaborators, transfer, clear values, edit existing fields or options, edit or delete comments, template marks | not in the binary |

A plan is a JSON file in the user state directory with the kit version, the creation and expiry time (thirty minutes), the actor login resolved from the API at creation, the host name, the project, the command that produced it, a PT-BR description for the team, and the steps: operation, target by node id with repository, number and title, the `before` value read at creation and the `after` value. The file carries a SHA-256 of its canonical content.

The MCP transport (`gh board mcp`, [mcp.md](mcp.md)) exposes the same tiers as tools and never `apply`, so a plan returned to an agent still needs a person in a terminal; the guards below stay necessary because raw `gh` from the agent's shell does not pass through the server.

`gh board apply <id|path>` refuses without an interactive terminal, when the hash does not match, when the plan expired, or when the current viewer login differs from the plan actor (exit code 6). It re-reads every target and stops before any write when a `before` value drifted (exit code 4), executes the steps in order writing one journal line per step, stops at the first failure so that a retry resumes rather than repeats, checks idempotency where GitHub cannot (`digest post` looks for a status update of the same ISO week; comments created by plans carry an invisible marker), and appends an audit line. A confirmation flag that authorizes the change does not exist, because an agent can pass a flag or allocate a pseudo-terminal (ADR-003).

Direct writes read the item immediately before the mutation. `--expect field=value` makes the write conditional on that current value; `move` is always conditional on the status it reports as current, so a concurrent change is refused with exit code 4 instead of overwritten. Every write accepts `--dry-run` and `--reason`.

## Level 3: guards in the agents

One embedded pattern source with three families: GitHub CLI write subcommands on projects, destructive GitHub CLI commands on issues, repositories and labels, and GraphQL or REST bodies naming a forbidden mutation or the DELETE method. The kit's own `apply`, `agent uninstall` and `guard uninstall` are in the list, so an agent cannot apply a plan or remove its guard. In the Claude Code rule form:

```
Bash(gh project close *)      Bash(gh project copy *)        Bash(gh project create *)
Bash(gh project delete *)     Bash(gh project edit *)        Bash(gh project field-create *)
Bash(gh project field-delete *) Bash(gh project item-add *)  Bash(gh project item-archive *)
Bash(gh project item-create *) Bash(gh project item-delete *) Bash(gh project item-edit *)
Bash(gh project link *)       Bash(gh project mark-template *) Bash(gh project unlink *)
Bash(gh issue delete *)       Bash(gh issue transfer *)      Bash(gh repo delete *)
Bash(gh label delete *)       Bash(gh api * -X DELETE *)     Bash(gh api * --method DELETE *)
Bash(gh api graphql *deleteProjectV2*)  and one rule per forbidden mutation listed above
Bash(gh board apply *)        Bash(gh board agent uninstall *) Bash(gh board guard uninstall *)
```

Reads (`gh project list`, `view`, `field-list`, `item-list`, `gh api graphql` queries) stay allowed, so nothing else in the developer's day changes, and the guards never prompt.

| Agent | Layers |
| --- | --- |
| Claude Code | the rules in `permissions.deny` of the user settings, one `Bash(...)` and one `PowerShell(...)` rule per pattern, which the engine evaluates before hooks and mods and applies to every subcommand, subshell and command substitution; a `PreToolUse` hook on `Bash` and `PowerShell` running `gh board guard check --agent claude`; the mod's `tool.call` hook on both tools with a fail-closed `.catch`. Three layers because a hook timeout does not block and a mod that fails to load protects nothing. The PowerShell rules exist because Claude Code on Windows runs commands through that tool when Git Bash is absent, and because a Bash-only deny set would switch the PowerShell tool off. |
| Cursor | a `beforeShellExecution` hook in `~/.cursor/hooks.json` running `gh board guard check --agent cursor` with `failClosed: true`. |
| Codex | a `PreToolUse` hook in `~/.codex/hooks.json` running `gh board guard check --agent codex` (exit code 2 blocks; the person trusts the hook once through `/hooks`), plus `~/.codex/rules/gh-board.rules` with forbidden prefix rules, because hook failures do not block in Codex. |

`gh board guard check` reads the agent's JSON on stdin, tokenizes the command, splits on the shell separators, strips the same wrappers Claude Code strips, and denies when any segment matches a pattern, answering in each agent's documented shape. It folds the case of the program name and of the HTTP method only: `GH`, `gh.exe`, `-X delete` and `--method=Delete` are the same command to it, while subcommands and GraphQL names keep their case because gh and the API are case sensitive. In normal mode it looks inside `sh -c`, `bash -lc`, `pwsh -Command`, `powershell -NoProfile -Command` and `cmd /c` wrappers, in the string and the argv form, and judges the inner command; strict mode denies the wrapper itself and path-qualified `gh` (POSIX or Windows paths, quoted or not). The rule layers of the agents are literal: the Claude Code deny rules and the Codex prefix rules carry `gh` and `gh.exe` with every case spelling of the DELETE method, and the hook is the layer that covers other spellings. `gh board doctor` verifies that the rules and hooks are installed and unchanged (by hash) and that every installed hook names the binary that is running.

On Windows the installed hook command is `powershell.exe -NoProfile -NonInteractive -InputFormat Text -OutputFormat Text -EncodedCommand <base64>`, a UTF-16LE script that pipes the hook JSON into the resolved binary and exits with its status; the outer command has no shell metacharacters, so Git Bash, cmd and PowerShell parse it alike whatever the extension path holds. Elsewhere the hook is the quoted absolute path followed by `guard check --agent <name>`.

The Codex fragment matches `^Bash$` so it applies only to shell calls. See [agent-contracts.md](agent-contracts.md) for the verified hook JSON, denial channels, execpolicy syntax and offline checks. A policy check or mocked plugin test alone does not prove a live agent loaded its guard.

### Guard limits

A deny rule does not match the program by absolute path, through `env`, `eval`, a script, an SDK, `curl` or a browser, and in normal mode a path-qualified `gh` goes unseen; the hook inspects the usual shell wrappers but not a file passed with `--input` or `@file`. A hook that times out does not block. None of the layers is a shell sandbox. These limits are the reason levels 1 and 2 exist: the plan is what binds a human's approval to concrete changes, and the binary has nothing destructive to run.

### Strict mode

`gh board agent install --agent <name> --strict` adds denies for `sh -c`, `bash -c`, `zsh -c`, `pwsh -c`, `powershell -Command`, absolute paths to `gh`, `env * gh *`, `xargs * gh *`, and `gh api graphql` with `--input` or `@file` bodies. It is off by default because it interferes with ordinary work; a team that wants it adopts it per person, and `gh board doctor` shows whether it is on.

## Level 4: what stays outside the kit

Written for the team, so that nobody assumes more than the kit delivers:

- Concurrent human edits between a plan and its apply: detected by drift, not prevented.
- Workflow changes made in the GitHub UI: the kit reads workflows but never edits them, and a workflow can close or move items on its own.
- Pull requests with closing keywords close issues without the kit.
- Raw API access through channels the guards do not see (absolute paths, shell wrappers, scripts, SDKs, `curl`, a browser).
- Prompt injection through issue text: the formatting below reduces it but cannot eliminate it.
- Local audit and journal files are editable by the same user: they are evidence, not proof.
- A developer token that GitHub lets delete things.

Mitigations the team can adopt beyond the kit: strict mode, agent sandboxes with network allowlists, and the review of active workflows by the PM when adopting the template.

## Level 5: reversible operations stay reversible

`restore` unarchives a project item and `reopen` reopens an issue; every direct write and every plan step journals the `before` and `after` values, so a change can be undone by hand from the journal.

## Identity, targets and permissions

- `doctor` and every write resolve the effective actor with a `viewer` query and report when `GH_TOKEN` or `GITHUB_TOKEN` shadows the keyring token (`GH_TOKEN` has precedence over `GITHUB_TOKEN` in `gh`). The kit never logs or prints the token, only its source.
- Before a write, the kit checks the viewer's permission on the target repository and the project role; a missing permission is reported before any mutation (exit code 3).
- Targets resolve to node ids. `#n` is accepted only with `repository` configured in `board.yml` and only when exactly one project item carries that number; otherwise the command asks for `owner/repo#n` or a URL (exit code 2).

## External text is data

`context`, `item`, `list` and `search` wrap every external text (titles, bodies, comments, README content) in explicit data delimiters, truncate titles to 120 characters and body excerpts to 280, strip control and ANSI sequences, and state the number of omitted items. The delimiters are kept in the formats the agents read (`compact`, `md` and `--json`); the `table` format, which is for people, prints the same cleaned and truncated text without them, and a text that tried to forge a delimiter stays broken there as well. The kit never writes external text into a system prompt. Plans show external text the same way.

## Local state hygiene

Files live in the per-OS config, state and cache directories that go-gh resolves, are created with owner-only permissions and written atomically (temporary file and rename). A lock file makes `watch` single-instance. Notification text is passed as arguments, never interpolated into a script. Audit and journal retention is ninety days, pruned by `watch`. Caches hold the schema only, never issue content.

State includes `plans/<id>.json`, `journal/<id>.jsonl`, `audit.jsonl`, `alerts.json`, `notified.json` and the watch lock. `alerts.json` is the current alert snapshot used for comparison with the next run; `notified.json` records notification send times so the hourly budget spans runs.

## board.yml cannot weaken anything

`board.yml` carries no permission or policy-weakening key. The guard patterns, the mutation allowlist and the verb tiers are compiled into the binary and cannot be changed by configuration. Decoding is strict, and `gh board doctor` prints which file is in use and its SHA-256, so a poisoned file is visible.

## If you suspect tampering

Run `gh board doctor`: it prints the configuration file and its hash, the state of every rule and hook against the expected hash, and the installed binary against the release checksum. Reinstall the agents with `gh board agent install` (idempotent, prints the diff) and review the local audit log with `gh board log --since <date>`.

### What the binary check proves

Doctor hashes the running binary and compares it with the digest GitHub records for the release asset of the same version and platform. When they match, `Verified: yes`. When they differ, doctor downloads that asset once into a temporary file, compares its SHA-256 with the manifest (`Provenance`) and inspects the Mach-O code signature of the installed file:

- On Apple silicon, `gh extension install` re-signs the downloaded binary with `codesign --sign - --force`, so the installed copy always differs from the asset by its signature blob and the three header fields that size it. A copy whose signature is ad hoc and not linker-signed is compared with the downloaded asset apart from those bytes: equal means `Verified: yes` with the note `installed copy re-signed by macOS after download`; anything else is a problem.
- Everywhere else, and for a signature the Go linker wrote, a difference is a problem: `checksum differs from the release checksum`.
- `release asset differs from the manifest` means the published asset no longer matches its recorded digest, which is a problem whatever the installed copy says.

The download is one GitHub request, only when the installed file differs, never for a development build. Doctor cannot verify the attestation or the Sigstore bundle by itself; the note names the commands that do, `gh attestation verify <asset> --repo cristianargotti/gh-board` and the checksum of the downloaded asset against `checksums.txt`, both documented in [releasing.md](releasing.md). Run them on the asset you download, not on the installed copy.
