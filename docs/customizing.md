# Customizing a board

The kit discovers the board by itself: fields, options, iterations, views, workflows, linked repositories and the viewer's role come from GraphQL on first use and are cached for one hour (`gh board schema --refresh` renews the cache; a write that fails on an unknown option renews it too). What discovery cannot know is what the fields mean to the team. That is `board.yml`: one optional file per team.

## board.yml, key by key

The reference file is `templates/board.yml` (an example team board, `acme/7`, with comments in PT-BR). Every key is listed here with its effect.

### Top level

| Key | Type | Meaning |
| --- | --- | --- |
| `version` | integer | File format version; only `1` is accepted. |
| `project.owner`, `project.number` | string, integer | The board. `--project owner/number` overrides it. |
| `repository` | `owner/name` | Where `new` creates issues and the default for the short form `#n`. Without it, items are addressed as `owner/repo#n` or by URL and `new` is unavailable. |
| `language` | string | Language of the content shown to the team (alerts, digest, plan descriptions); the reference uses `pt-BR`. |
| `timezone` | IANA name | Used for business days, sprint ends and the digest week. |

### template

Declarations used by the init copy plan, separate from capability mappings:

| Key | Type | Meaning |
| --- | --- | --- |
| `template.labels` | list of `{name, color, description}` | Repository labels to create when absent; `color` is six hexadecimal digits without `#`. The reference declares `entrada`, `urgente`, `lab`, `erro`, `oportunidade`, `regra` and `sinal`. |
| `template.milestones` | list of `{title, due_on, description}` | Repository milestones to create when absent; an optional `due_on` uses RFC3339, for example `2026-12-01T00:00:00Z`. The reference has an empty list so each team chooses its own commitments. |

### capabilities

A capability binds a role to a field, a label or an issue type. A capability that is absent is unavailable: the commands that need it say "unavailable: not mapped in board.yml" instead of guessing.

| Key | Fields | Used by |
| --- | --- | --- |
| `status` | `field`; `backlog`, `ready`, `active`, `done` (lists of option names) | `move`, `set`, `context`, `status`, `attention`, `tidy`, WIP and transition policy. Moving to a `done` option closes the issue through the board workflow, so it is a plan. Options not listed are shown as they are, without a class. |
| `epic` | `issue_type` (optional), `field` | `new epic`, `epics`, `roadmap`, `link --parent`, `tidy` (inheritance and epic status), the `epic_without_dates` alert. Without `issue_type` (a user-owned board has no issue types) the epic field, `--epic` and `new --epic` work and `new epic` creates a plain issue; `epics`, `roadmap` and the alert need the type to tell an epic apart. |
| `task` | `issue_type` (optional), `max_estimate_days` | `new task`, `estimate`. Without `issue_type`, `new task` creates a plain issue. |
| `lane` | `field` | `list --lane`, `new --lane`, `tidy` inheritance, the digest. |
| `sprint` | `field` (an iteration field) | `sprint`, `sprint set`, `list --sprint`, `context`, `tidy` (`sprint_from_target`), the `sprint_ending` alert. |
| `estimate` | `field` (a number field) | `estimate`, `new --estimate`. |
| `dates` | `start`, `target`, `source` (`issue_fields` or `project`) | `dates`, `new --start --target`, `roadmap`, `epics`, `item`, the `overdue` and `epic_without_dates` alerts, `tidy`. `issue_fields` are organization issue fields: a change is visible in every project that shows the issue, and plan descriptions say so. `project` means project date fields. |
| `blocked` | `field` | `list --blocked`, `attention`, `standup`, the `blocked` alert (also raised when the issue has blockers). |
| `triage` | `label`, `decision_field`, `sla_business_days`, `urgent_label` | `new entry`, `list --triage`, `context`, `attention`, the `triage_sla` alert (urgent items are due the same day), the digest metrics. |
| `lab` | `label`, `gate_field`, `result_field` | the lab section of the rituals and the digest. |

### policy

| Key | Meaning |
| --- | --- |
| `transitions` | Map from a status option to the options it may move to. A move outside the map is refused with exit code 3. Options not in the map accept any transition. |
| `wip` | Map from a status option to the maximum number of open items in it. A move that would exceed it is refused with exit code 3 and the `wip_exceeded` alert fires. |
| `bulk_threshold` | Number of items above which a direct write becomes a plan. |

### tidy

| Key | Meaning |
| --- | --- |
| `inherit_from_parent` | Roles a task inherits from its parent epic when empty: `lane`, `epic` or both. |
| `sprint_from_target` | Set the sprint from the target date when the sprint is empty. |
| `stamp_start_on_active` | Set the start date when an item entered an active status, from the status value timestamp. |
| `epic_follows_tasks` | Move the epic status following its tasks. |

### alerts

A map from rule id to options. An empty mapping (`{}`) enables the rule with its defaults; an unknown id is an error. `days` is the window for the rules that take one.

| Rule | Fires when | Needs |
| --- | --- | --- |
| `overdue` | the target date passed and the item is open | `dates` |
| `blocked` | the blocked field is set or the issue has blockers | `blocked` |
| `triage_sla` | a triage item has no decision after `sla_business_days` business days (urgent: same day) | `triage` |
| `wip_exceeded` | a status holds more open items than its WIP limit | `status` |
| `sprint_ending` | the sprint ends within `days` days | `sprint` |
| `stale_active` | an active item's status value is older than `days` days | `status` |
| `epic_without_dates` | an epic has no start or target date | `epic`, `dates` |

### digest

| Key | Meaning |
| --- | --- |
| `weekday` | The day the digest is published. |
| `metrics` | Any of `first_response`, `autonomy`, `run_share`. A metric without evidence prints "indisponível" with the reason. |
| `members` | The logins of the team, validated against the project members. |

### rituals

A list of `{ name, when, reads }`. `name` and `when` are free text shown to the team; `reads` names the sections the ritual reads (for example `active`, `attention`, `triage`, `epics`, `roadmap`, `lab`, `deferred`). `context` prints the rituals among the team rules.

## Rules of the file

- Decoding is strict: unknown keys, unknown capability names, unknown alert ids, unknown digest metrics and an unsupported version are errors with a one-line message.
- Every field, option, label and login named in the file must exist on the board. `gh board doctor` lists each mismatch with the YAML path, the value, a "did you mean" when a close name exists, and the commands that depend on it. An alert or tidy step that references a capability the file does not declare is reported too.
- The file carries no permission or policy-weakening key. Guard patterns, the mutation allowlist and the verb tiers are compiled into the binary.
- `repository` is required for `new` and for the short form `#n`.

## Resolution order

1. `--config <path>`
2. the environment variable `GH_BOARD_CONFIG`
3. `board.yml` in the working directory, walking up to the git root (without a git root only the working directory is searched)
4. `<user config dir>/gh-board/<owner>-<number>.yml` for the project of `--project`
5. the default recorded by `gh board use owner/number` in `<user config dir>/gh-board/default.yml`, with its per-user file when one is named after it
6. a single per-user file, when exactly one exists in `<user config dir>/gh-board`
7. generic mode

A path named explicitly by the flag or the variable must exist; a typo there is a usage error, never a silent fall through. `--project owner/number` overrides the project of any file. Two or more per-user files without a default keep asking for `--project` and name the candidates. `gh board doctor` prints which file is in use and its SHA-256, and the default project with the file it came from.

`GH_BOARD_HOME` roots the config, state and cache directories under one directory instead of the per-OS go-gh directories; tests and CI use it.

## Generic mode

Without a file every read command works: fields are shown by their real names, no role is assumed, `move` accepts any option of the field named exactly `Status` when it exists, and every capability that depends on a role reports "unavailable: not mapped in board.yml". Writes that need a role (`new`, `sprint set`, `estimate`, `dates`, `tidy`) are unavailable until the role is mapped.

## What init infers

`gh board init` plans a draft `board.yml`; the human writes it by applying the plan. The draft uses discovery and maps only unambiguous facts:

- the field named exactly `Status` becomes `capabilities.status.field`, with every option listed in a YAML comment for a person to distribute among `backlog`, `ready`, `active` and `done`;
- a single iteration field becomes `sprint`;
- fields named exactly `Start date` and `Target date` become `dates`, with the source detected from the field kind (project date fields, or organization issue fields);
- the issue types present on the organization are listed as candidates for `epic` and `task` but not assigned.

Everything else is left absent and reported as unavailable in a trailing comment. There is no "last option is done" and no name similarity heuristic. The draft decodes strictly, so it can be used as is and completed by hand.

## Board structure and views

The kit maintains the data the board is organized by, and the views keep rendering from it:

| Structure | Data the kit maintains | Verbs |
| --- | --- | --- |
| epics | the configured issue type plus the epic field | `new epic`, `epics`, `roadmap` |
| tasks | sub-issues of an epic, with sub-issue progress | `new --parent`, `link --parent`, `item`, `epics` |
| lanes | `capabilities.lane`, inherited from the parent by `tidy` | `new --lane`, `set`, `tidy` |
| sprints | the iteration field | `sprint`, `sprint set`, `new --sprint` |
| milestones | repository milestones | `milestone create` and `milestone retarget` as plans, `milestone set` and `new --milestone` as direct writes |
| dates | `capabilities.dates`, with their source | `dates`, `new --start --target` |

Views are not edited by the kit: the roadmap view, its markers (iterations, milestones and date fields) and the insights charts arrive with the template copy and keep rendering from the data above. GitHub exposes no API to read insights data or to create charts, so the kit's own measurements come from the items: `status` counts, `epics` and `roadmap` progress, `attention`, and the digest metrics.

## Issue forms

After the human applies its plan, `gh board init --forms` renders the forms in `templates/forms/` into `.github/ISSUE_TEMPLATE/` beside the draft selected by `--output` (the working directory by default) for a pull request. The forms are written with the names of the reference `board.yml`; rendering replaces each reference name with the value of the same key in the team's file (the table in `templates/README.md` lists the pairs). The renderer must preserve the PT-BR form bodies, substitute configured field names and labels, update the board URL, and set only issue types available on the target organization. If the organization has no issue types, it drops the top-level `type` field. When `.github/` has code owners in the repository, the pull request needs their approval.
