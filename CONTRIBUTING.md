# Contributing

The repository has a small set of laws. `make ci` checks every one of them that a machine can check, and the CI workflow runs the same target on macOS, Linux and Windows.

## Laws

- `make ci` passes before a commit is proposed: `gofumpt`, `golangci-lint`, the repository laws, `govulncheck`, `gitleaks`, the race tests, the coverage gate and the build. The pre-commit hooks in `lefthook.yml` run the fast part of it.
- Coverage is 80 percent per file, without exemptions. A new file brings its tests.
- Go files have at most 300 lines and functions at most 40 lines (`go run ./tools/laws -root .`). Split a file before it grows past the limit.
- No em dashes and no en dashes in any text file, source and documentation included. Use a comma, a colon or a hyphen.
- Commits are conventional and in English: `feat(scope): what changed`, `fix`, `docs`, `test`, `build`, `ci`, `chore`, `refactor`, `perf`, with `!` for a breaking change. No trailers of any kind: no sign-offs, no co-author lines, no generated-with lines.
- Board content shown to the team (template, forms, alerts, digest, plan descriptions) is in PT-BR; everything else, code comments included, is in English.
- `agents/SKILL.md` is the single source of the skill; copy it verbatim to `agents/claude/skills/gh-board/SKILL.md` after editing it (`go test ./` checks the copy and the embedded asset tree).
- A new mutation enters the allowlist in `internal/github` only with a verb that needs it and the review that goes with it; the source scan test fails on any other mutation.
- A new third-party dependency needs a decision record. The maintainer keeps the decision records (ADR-001 and up) with the design specification; the documentation cites them by number.

## Proposing a change

1. Open an issue that states the problem and the verb or page it touches.
2. Branch from `main`, keep the change small, run `make ci` and `make acceptance`.
3. Open a pull request with the conventional title; the CI matrix, the agent acceptance, the release configuration check and the Claude Code plugin validation must be green.
