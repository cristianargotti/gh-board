# Commands

Every command is `gh board <verb>`; `gh board --help` and `gh board <verb> --help` print the same catalog from the binary. Verbs are grouped by tier because the tier decides what a call may do: read verbs have no side effects, direct writes change one item reversibly, plan verbs write a file that nothing executes until a human runs `apply`, and the automation verbs manage the scheduler and the agents.

## Global flags

| Flag | Applies to | Meaning |
| --- | --- | --- |
| `--project owner/number` | every verb | the board; overrides the project of any `board.yml` |
| `--config path` | every verb | the `board.yml` to use, first in the resolution order |
| `--json` | every verb | structured output with the same fields as the text form |
| `--format table\|compact\|md` | every verb | text output format; `table` is the default. `table` is for people and shows external text (titles, bodies, values) cleaned and truncated but without the data delimiters; `compact`, `md` and `--json` keep the `[[begin:title]] ... [[end:title]]` delimiters for the agents |
| `--dry-run` | writes, `apply` | show what would be sent and send nothing |
| `--reason text` | writes | a reason stored in the audit line |
| `--expect field=value` | writes to existing items | precondition: the write is refused with exit code 4 when the current value differs; repeatable |

`--format` is validated before anything is read, so a wrong value fails in milliseconds.

No command needs `--project` once a default is recorded with `gh board use owner/number`: the project comes from the flag, then `GH_BOARD_CONFIG` or `--config`, then the nearest `board.yml`, then the per-user file of the project, then the recorded default, then a single per-user file when exactly one exists. Two or more per-user files without a default stop with "no project selected" and name the candidates.

## Timestamps and dates

Every `--json` payload, plan file, journal line and audit line carries instants as RFC 3339 in UTC at second precision (`2026-01-15T09:30:00Z`); the text formats print the same form. Calendar dates (sprint start and end, milestone due dates, start and target dates) are `YYYY-MM-DD` and never shift with the time zone: GitHub returns them as midnight UTC and the kit prints them as they are. "Today" for overdue counts, the current sprint and days left is the date in the `timezone` of `board.yml` (UTC without one).

## References

A `<ref>` names an issue or a project item in one of these forms:

| Form | Example | Notes |
| --- | --- | --- |
| node id | `I_kwDO...`, `PVTI_...` | always unambiguous |
| full | `owner/repo#17` | works without `board.yml` |
| short | `#17` or `17` | needs `repository` in `board.yml` and exactly one project item with that number; otherwise exit code 2 asks for the full form or a URL |
| URL | `https://github.com/owner/repo/issues/17` | pull request URLs are accepted too |

A `<status>` is an option name of the status field, as the board spells it (`"IN PROGRESS"`, `"TEST / VALIDATION"`). A `<login>` is a GitHub login. Names are validated against the discovered schema before anything is sent, with a "did you mean" suggestion on mismatch.

## Read

Items are read fresh on every command, in pages of 100, and never cached. `context`, `status`, `me`, `list`, `epics`, `sprint`, `attention`, `standup` and `watch` read a lighter selection of each item (no body, milestone or parent, and the issue field values read once, from the issue), which is what they show; `item`, `search`, `roadmap`, `digest` and every write read everything.

| Command | What it prints |
| --- | --- |
| `context [--budget N] [--for login]` | The snapshot for a prompt: team rules (flow, WIP, SLA, rituals), sprint and days left, my open items, attention, triage queue, epics progress, deliveries of the week, fitted to the budget (about 1,500 tokens by default). Every section gets a share first: my items at most 10, most urgent first (the overdue ones, then by target date, then the most recently moved); attention at least 8 and at most 12, most severe first; the triage queue up to 5, oldest first; the epics all when they fit or at least 6, with progress; the deliveries up to 6. When the shares do not fit, the epics shrink to 6 and attention to 8 and never below: the shares are kept even when the budget is smaller than them, so a section with content never prints "none"; when they fit, the rest of the budget shows more of my items. The budget is measured on the bytes of the format being printed, so the output fits it in every format, and the `omitted` counts stay exact per section. Ends with `generated_at`, `completeness` (`complete` or `partial`), `omitted` counts per section and `omitted_items`, the board items left out because they are not issues. |
| `status` | Board overview. The item counts name the board items omitted because they are not issues (draft issues, pull requests, redacted items). |
| `me` | My open items. |
| `item <ref>` | One issue with its board fields, parent, sub-issue progress, blockers, and dates with their source. Free text values (the title and text fields) arrive in the data delimiters like the issue title and body. |
| `list [filters] [--all]` | Items. Filters: `--status`, `--assignee`, `--lane`, `--epic`, `--sprint`, `--label`, `--type`, `--overdue`, `--blocked`, `--triage`, plus `--limit N` for the page size. `--all` paginates to the end instead of the first page; without it a filter applies to the first page only, and the footer says how many items the board holds. `omitted` counts the board items that are not issues. |
| `epics` | Epics with progress, dates and owner. |
| `roadmap [--months N]` | Epics and milestones on a month timeline with the sprint iterations as markers, from the same dates the GitHub roadmap view reads; table, markdown or JSON. |
| `sprint current`, `sprint next` | The sprint: title, start, end (the last day of the sprint, as GitHub shows it), days left (zero on the last day), open and done counts. |
| `attention` | Overdue, blocked, triage past SLA, WIP exceeded. |
| `search <query>` | Items matching the query, with the count of board items that are not issues. |
| `schema [--refresh]` | The discovered schema: fields with options and iterations, views, workflows, linked repositories, viewer role. The schema is cached for one hour; `--refresh` discards the cache. |
| `doctor` | Identity and token source, configuration file in use and its SHA-256, `board.yml` names the board lacks with the commands that depend on them, agent rules and hooks (presence and hash), the binary every installed hook names against the running one, binary checksum against the release. |
| `log [--since date]` | The local audit log; targets are named as `owner/repo#n` when the entry knows them, node ids otherwise. |
| `plan list`, `plan show <id>` | The plans in the state directory, and one plan in full with its steps and expiry. |
| `digest print [--week ISO]` | Prints the weekly digest in PT-BR (deliveries, milestones, risks, the metrics of `board.yml` with their evidence) without posting it. |
| `standup [--for login]` | Per person: moved yesterday, active today, blocked. |
| `version` | The kit version. |

## Direct write

Direct writes read the item immediately before the mutation and are journaled. Commands whose help shows `<ref[,ref...]>` accept comma-separated references; above `policy.bulk_threshold` unique items they become a plan. `new` rejects `--expect` because the issue does not exist yet. Put `--` before label arguments when removing a label so that `-name` is an argument, not a flag.

| Command | What it does |
| --- | --- |
| `new task\|epic\|entry` | Creates the issue in `repository` with the configured issue type when one is mapped (a plain issue on a board whose owner has no issue types), adds it to the project, sets the fields and links the parent, journaled as one plan-shaped record executed immediately. Flags: `--title`, `--body` (text, a file path, `@path` or `-` for stdin), `--parent`, `--lane`, `--epic`, `--sprint`, `--estimate`, `--start`, `--target`, `--milestone`, `--assignee`, `--label`. `entry` creates an item for the triage queue with the triage label. |
| `move <ref> <status>` | Validates the transition and the WIP limit, then writes, conditional on the status it reports as current. A refused transition names the destinations `board.yml` allows (exit code 3). A done status turns the call into a plan (exit code 5). |
| `assign <ref> <login>`, `unassign <ref> <login>` | Adds or removes an assignee. |
| `set <ref> field=value ...` | Sets one or more fields. Non-status fields directly; the status field through the same policy as `move`. |
| `sprint set <ref> current\|next\|<title>` | Sets the iteration field. |
| `estimate <ref> <days>` | Sets the estimate field, within `capabilities.task.max_estimate_days`. |
| `dates <ref> [--start YYYY-MM-DD] [--target YYYY-MM-DD]` | Sets the start and target dates on the configured source. With `source: issue_fields` the change is visible in every project that shows the issue. |
| `milestone set <ref> <title>` | Sets the issue milestone through `updateIssue`. |
| `comment <ref> <text\|file\|->` | Adds a comment. The argument is read as a file only when it is `-` (stdin), `@path`, or names an existing file; any other text is the comment itself, so punctuation never turns a comment into a file name. Trailing line breaks are dropped. |
| `link <child> --parent <epic>` | Makes the child a sub-issue of the epic; the record names the parent as `owner/repo#n`. |
| `label <ref> -- +name -name` | Adds the labels prefixed with `+` and removes those prefixed with `-`. |
| `reopen <ref>` | Reopens a closed issue; an open issue is reported as nothing to do and nothing is written or journaled. |
| `restore <ref>` | Unarchives a project item; an active item is reported as nothing to do. |

The record of a direct write describes what the verb did (the step descriptions in PT-BR), so `plan list` tells a move from a label change.

## Plan

Plan verbs write a JSON plan in the state directory and change nothing. The output names the plan id and the exact `gh board apply <id>` to run. A plan expires thirty minutes after creation.

| Command | What the plan does |
| --- | --- |
| `close <ref>` | Closes the issue. |
| `move <ref> <done status>` | Moves to a done status, which closes the issue through the board workflow. |
| `digest post [--status on_track\|at_risk\|off_track]` | Publishes the week as a project status update, once per ISO week. |
| `tidy` | The routine steps of `board.yml`: inherit lane and epic from the parent epic, sprint from the target date, start date when an item entered an active status, epic status following its tasks. |
| `init [--from owner/number] [--repository owner/name] [--owner login] [--title text] [--output path] [--forms]` | Plans a template copy when `--from` is set, the missing `template.labels` and `template.milestones`, and the repository link. Without `--from`, uses the selected project. The repository comes from `--repository` or configuration. After apply, writes a draft to `--output` (default `board.yml`) and, with `--forms`, forms under `.github/ISSUE_TEMPLATE/` next to that draft. Existing changed files are refused. |
| `milestone create <title> [--due YYYY-MM-DD] [--description text]` | Plans creation of a milestone in the configured repository. The due date is sent as noon UTC of that day, because GitHub keeps the day of the instant in US Pacific time and midnight UTC would land on the day before. |
| `milestone retarget <from-title> <to-title>` | Plans reassignment of non-archived board issues in the configured repository from one existing milestone to another. It does not edit either milestone deadline. |
| any direct verb over more than `policy.bulk_threshold` items | The same writes, as a reviewable plan. |

## Apply

`apply <id|path> [--dry-run]` executes a plan. It needs an interactive terminal (on Windows a Git Bash or mintty pseudo terminal counts, as it does for `gh`) and refuses when the hash does not match, the plan expired or the current login differs from the plan actor (exit code 6); re-reads every target and stops before any write when a `before` value drifted (exit code 4); executes the steps in order with one journal line per step, so a failed plan resumes rather than repeats; and appends an audit line. The agent guards deny `gh board apply`, so a person runs it.

## Automation

| Command | What it does |
| --- | --- |
| `use [owner/number] [--clear]` | Records the default project in `<config dir>/gh-board/default.yml` after confirming it exists, so every command reads it when no `--project`, `GH_BOARD_CONFIG` or `board.yml` selects one. Without an argument it prints the current default and its file; `--clear` forgets it. `doctor` prints the default and where it came from. |
| `watch [--once]`, `watch --install [--interval 10m]`, `watch --uninstall` | These modes are mutually exclusive. The default is one evaluation: write `alerts.json` and track delivered notifications in `notified.json`. Installation requires a project from `--project owner/number` or configuration. It registers the native scheduler as the user (launchd, systemd user timer or cron, Task Scheduler); uninstall removes it. On Windows the task runs in the person's session without a stored password, which keeps the keyring and the network, and through `conhost.exe --headless` so no console window opens at every interval. Audit and journal retention is ninety days. |
| `agent install --agent claude\|cursor\|codex\|all [--strict] [--scope user\|project] [--mcp]` | Installs the skill and guards; idempotent, prints a diff before writing. `--strict` adds strict denies; `--scope project` adds repository files; `--mcp` also registers the MCP server entry. Claude Code also needs the marketplace plugin installation in [setup.md](setup.md). |
| `agent uninstall --agent claude\|cursor\|codex\|all [--scope user\|project] [--mcp]` | Removes only the installer-owned content; with `--mcp`, only the MCP server entry. |
| `guard check --agent claude\|cursor\|codex [--strict]` | Reads the agent's JSON on stdin and answers allow or deny in that agent's shape. This is what the hooks call. |
| `guard install --agent claude\|cursor\|codex\|all [--strict] [--scope user\|project]` | Installs the guard layer alone. |
| `guard uninstall --agent claude\|cursor\|codex\|all [--scope user\|project]` | Removes the guard layer alone. |
| `mcp [--project owner/number] [--config path]` | Serves the kit as an MCP server over stdio: the read verbs, the direct writes and the plan verbs `close`, `tidy` and `digest post` as tools, with the same rules and exit codes; `apply` is never a tool. `agent install --mcp` registers it. See [mcp.md](mcp.md). |

## Absent

No verb deletes or archives anything, removes an item from a project, unlinks a repository, edits views, workflows, insights, visibility, settings or collaborators, transfers an issue, clears a value, edits existing fields or options, edits or deletes comments, or marks a template. These operations are not in the binary; see [security.md](security.md).

## Exit codes

| Code | Meaning | Typical cause |
| --- | --- | --- |
| 0 | success | |
| 1 | usage | unknown verb or flag, malformed reference, invalid `board.yml` |
| 2 | not found or ambiguous | the reference matches no item, or `#n` matches more than one |
| 3 | policy refused | transition not allowed, WIP limit reached, missing permission |
| 4 | drift detected | an `--expect` value or a plan `before` value differs from the current one |
| 5 | plan required | the verb wrote a plan instead of executing (done status, bulk above the threshold) |
| 6 | apply refused | no interactive terminal, expired plan, hash mismatch, actor differs |
| 7 | GitHub API error | network, rate limit, GraphQL error |
