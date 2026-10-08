# Replay fixtures of the GitHub adapter

Every file under `replay/<set>/` is one recorded exchange: the operation
name and the variables that identify it under `request`, the status,
headers and body under `response`. `replay_test.go` serves them over
`httptest`; no test reaches the network.

| Set | Source | Content |
| --- | --- | --- |
| `acme` | an organization board (`acme/7`, repository `acme/app`), read-only queries | discovery, four item pages, archived page, item by node and by number, comments, labels, milestones, issue types, viewer, permission, issue field kinds, issue by id, user and owner ids, repository id, assignable users, organization issue fields, status updates, timelines (one with issue field events in pages of two), project by id |
| `template` | an organization template board (`acme/24`) | discovery and the empty item page |
| `devops` | a second organization board (`acme/26`) | discovery and one item page |
| `sec` | a third organization board (`acme/35`) | discovery and one item page |
| `sandbox` | user project 2 and repository gh-board-sandbox | discovery of a user-owned board, permission, labels, milestones, issue types of a user owner, status updates, timeline and item of the test issue, assignable users, repository id, project by id, issue by id |
| `sandbox-writes` | the 22 allowlisted mutations run once in the sandbox | the real answer of every mutation, the REST milestone, and the error shapes of an unknown issue type and of organization issue fields on a user repository |
| `errors` | recorded (401, missing project, missing owner) and handcrafted (missing project page, missing repository, odd timeline shapes, an issue without items) | error shapes |
| `ambiguous` | derived from `acme` | two issues with the same number on one board |
| `release` | public repository cli/cli, read-only | the assets and digests of one release and a missing release, for the doctor checksum |

Sanitization, applied before the files entered the repository: issue
titles, bodies, comment bodies and the README replaced with synthetic
PT-BR text; member logins replaced with `membroN` (bots kept); the
owner, the repository names, the board titles and the team specific
option and label names (lanes, epics, themes, teams) replaced with
generic names; node ids randomized with their prefixes kept, consistently
across every file; item pages trimmed to twelve items. Structure,
pagination cursors, field names, the remaining option names, iteration
titles, views, workflows and dates are the real ones. The handcrafted
files in `sandbox-writes` are `issue_field_fake_date.json`,
`set_issue_field_value.json` and `update_issue_field_value.json`, built
from the schema because a user repository cannot carry organization
issue fields; `errors` adds the handcrafted `issue_timeline_odd.json`,
`item_by_node_odd.json` and `issue_timeline_orphan.json` for shapes the
live boards never produced. Legacy user node ids in `assignable_users`
were replaced by hashed ids.
