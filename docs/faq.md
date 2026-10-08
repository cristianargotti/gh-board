# FAQ

### Why a CLI extension and not an MCP server?

One binary serves people, schedulers and the three agents, using each person's existing `gh auth`. It works from any terminal and from cron, and the agents learn it through a skill and `--help`. The MCP transport exists too, as another door to the same core: `gh board mcp` serves the verbs as tools with the same rules, and `gh board agent install --mcp` registers it (ADR-005, [mcp.md](mcp.md)).

### Why is there no confirmation flag for closing or moving to done?

An agent can pass a flag or allocate a pseudo-terminal, so a flag binds nothing. Guarded operations write a plan with the targets, the before and after values, the actor and an expiry; only `gh board apply <id>` executes it, in an interactive terminal, after re-reading the preconditions, and the agent guards deny `apply`. One extra human command for the rare consequential changes (ADR-003).

### Can the agent delete something through the kit?

No. The binary has no code path that deletes, archives, removes, unlinks, reconfigures or changes visibility or collaborators; a source scan test fails the build if one appears. The guards also deny the raw `gh project delete`, `gh issue delete`, `gh api -X DELETE` and the forbidden GraphQL mutations inside the agents. What the guards cannot see (absolute paths, `sh -c`, scripts, SDKs, `curl`, a browser) is listed in [security.md](security.md); a developer token that GitHub lets delete things is the residual risk the team must know.

### What does "unavailable: not mapped in board.yml" mean?

The command needs a role (sprint, epic, lane, dates, triage, lab, blocked, estimate) that no `board.yml` declares. The kit never guesses roles from field names or option order; declare the capability in `board.yml` and run `gh board doctor`. See [customizing.md](customizing.md).

### Which exit code means what?

0 success, 1 usage, 2 not found or ambiguous target, 3 policy refused (transition, WIP, permission), 4 drift detected, 5 plan required, 6 apply refused (no terminal, expired, hash, actor), 7 GitHub API error. The table with causes is in [commands.md](commands.md).

### `move` returned exit code 5. Is that an error?

No. The status is a done status, so the move closes the issue through the board workflow and became a plan. The output names the plan and the exact `gh board apply <id>` to run.

### `move` returned exit code 4. What happened?

Someone changed the item between the read and the write: `move` is always conditional on the status it reports as current, and `--expect field=value` makes any write conditional. Read the item again and decide.

### I have `GH_TOKEN` set. Which identity does the kit use?

The one `gh` uses: `GH_TOKEN` first, then `GITHUB_TOKEN`, then the keyring token. `gh board doctor` and every write report the token source, because writes are attributed to that identity on GitHub. The kit never prints the token itself.

### Where are the local files?

In the per-OS directories go-gh resolves: configuration (per-user `board.yml` copies), state (plans, `journal/`, `audit.jsonl`, `alerts.json`, `notified.json`, the `watch` lock) and cache (the board schema only). Files are owner-only and written atomically; audit and journal entries are pruned after ninety days. `GH_BOARD_HOME` roots the three under one directory.

### Does the kit cache issue content?

No. The cache holds the schema only, for one hour. Items are never cached across invocations and every write re-reads the item it touches.

### How does the kit handle issue text given to the agent?

As data. `context`, `item`, `list` and `search` wrap external text in explicit delimiters in the formats the agents read (`compact`, `md`, `--json`), truncate titles to 120 characters and body excerpts to 280, strip control and ANSI sequences and state how many items were omitted; the `table` format, which is for people, prints the same cleaned text without the delimiters. Nothing external is written into a system prompt. Prompt injection is reduced, not eliminated; the residual risk is written down.

### Can I change the deny rules or the allowlist in board.yml?

No. `board.yml` carries no permission or policy-weakening key; the guard patterns, the mutation allowlist and the verb tiers are compiled into the binary. `board.yml` describes what the fields mean to the team, nothing else.

### Strict mode breaks my `sh -c` and `env` invocations. Is it required?

No. Strict mode is opt-in per person (`gh board agent install --agent <name> --strict`) and off by default precisely because it interferes with ordinary work. The base guards deny only the listed destructive commands and never prompt.

### Does the Claude Code mod poll GitHub?

No. It reads `alerts.json`, which `gh board watch` writes, and refreshes on `/board`. Without `watch` installed it shows "sem watch instalado" and offers the install command. Toasts are limited to five per hour; the rest go to the pane.

### What does `init` copy from the template?

GitHub copies views, custom fields, workflows except auto-add, and insights; it does not copy items, collaborators or repository links. The plan then creates missing labels and milestones declared under `template` in the source `board.yml`, links the team repository and writes the draft `board.yml`. `init` never modifies existing fields or options.

### The forms pull request is blocked. Why?

When `.github/` has code owners in the repository (a CODEOWNERS rule, common in organizations), the pull request that adds `.github/ISSUE_TEMPLATE/` needs their approval.

### Can I use the kit with several boards?

Yes. One `board.yml` per repository checkout (found from the working directory up to the git root), a per-user file `<config dir>/gh-board/<owner>-<number>.yml` for boards without a checkout, or `--project owner/number` on the command line, which overrides any file. `gh board use owner/number` records the board you work on most, so the commands need no flag; a single per-user file is also picked on its own.

### Does the kit edit views, workflows or insights?

No. Views, workflows and insights have no code in the binary. They arrive with the template copy and keep rendering because the kit maintains the data they read: epics, sub-issues, lanes, sprints, milestones and dates.

### Does it run on Windows?

Yes, as a first-class platform: precompiled binaries for amd64 and arm64, paths from go-gh, Task Scheduler for `watch --install` (in the person's session, without a stored password, through the headless console host so no window flashes), a PowerShell toast for notifications, and hooks that call the binary by its resolved path through an encoded PowerShell invocation that Git Bash, cmd and PowerShell parse alike. `gh board apply` accepts a Git Bash or mintty pseudo terminal as well as a console. Claude Code on Windows may run commands through its `PowerShell` tool: the deny rules, the hook and the mod cover it. The guard matches `gh` and `gh.exe` in any letter case; the rule layers of the agents (Claude Code deny rules, Codex execpolicy) are literal and cover `gh` and `gh.exe`, the kit's own `guard check` covers the rest. Native Windows runs are part of the release checklist, not of the CI matrix of the guard tests.

### How do I update?

`gh extension upgrade gh-board`. The skill and the hooks state the minimum kit version; `gh board doctor` compares the installed binary with the release checksum and reports the state of the installed rules and hooks.

### Who may install it?

Anyone with the GitHub CLI: `gh extension install cristianargotti/gh-board`. Releases are signed with Sigstore and carry GitHub artifact attestations, checksums and an SBOM; `gh board doctor` compares the installed binary with the release checksum. An organization that wants its own release line forks the repository and reviews the release workflow before tagging.
