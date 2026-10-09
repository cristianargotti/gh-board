# Changelog

## 0.1.1

- `doctor` no longer reports a legitimate install on Apple silicon as tampered: `gh extension install` re-signs the downloaded binary with `codesign`, so the installed copy differs from the release asset by its code signature. Doctor now inspects the Mach-O signature, downloads the release asset of the running version and platform once when the installed file differs, checks it against the manifest (`Provenance: release asset matches the manifest`), compares a re-signed copy with it apart from the signature, and names the two verifications a person can run on the asset: `gh attestation verify <asset> --repo cristianargotti/gh-board` and the checksum against `checksums.txt`.
- Releases carry one SPDX JSON SBOM per binary again. The SBOM catalog named the build id, which matches none of the uploadable binaries of the binary archive format; it now names the archive id, and `tools/cicheck` checks it.
- `docs/releasing.md` explains how to verify a download (attestation with `--repo`, `cosign verify-blob` with the bundle, the checksum manifest) and the macOS re-signing; `docs/security.md` states what the binary check proves.

## 0.1.0

The first public release of `gh board`, a GitHub CLI extension that lets a team run a GitHub Projects v2 board with AI coding agents.

- Read verbs (`context`, `status`, `me`, `item`, `list`, `epics`, `roadmap`, `sprint`, `attention`, `search`, `schema`, `standup`, `digest print`, `doctor`, `log`, `plan list`, `plan show`, `version`) that read the board fresh, in pages of 100, and fit `context` to a token budget with a share per section.
- Direct writes (`new`, `move`, `assign`, `unassign`, `set`, `sprint set`, `estimate`, `dates`, `milestone set`, `comment`, `link`, `label`, `reopen`, `restore`) that read the item first, honor the flow policy and the WIP limits, accept `--expect`, `--dry-run` and `--reason`, and are journaled.
- Plans (`close`, `move` to a done status, `tidy`, `digest post`, `init`, `milestone create`, `milestone retarget`, bulk writes above the threshold) that a person executes with `gh board apply` in an interactive terminal after the preconditions are re-read.
- A closed mutation allowlist with a source scan test: nothing in the binary deletes, archives, hides or reconfigures anything.
- `board.yml`: one optional file per team that maps what the fields mean (status classes, epic, task, lane, sprint, estimate, dates, blocked, triage, lab), the flow policy, the tidy steps, the alerts, the digest and the rituals, with strict decoding and `doctor` reporting what the board lacks.
- A board template with PT-BR issue forms, labels and the `init` plan that copies a template board, creates the labels, links the repository and writes a `board.yml` draft.
- Alerts (`watch`) through the native scheduler of each operating system, with desktop notifications and no server, bot or shared token; a weekly digest with evidence-backed metrics.
- The agents kit: a skill, deny rules and PreToolUse hooks for Claude Code, Cursor and Codex CLI, a Claude Code marketplace plugin with a `/board` pane, an opt-in strict mode, an installer that is idempotent and prints a diff, and `doctor` checks of every installed artifact and hook.
- An MCP transport: `gh board mcp` serves the read verbs, the direct writes and the plan verbs as tools over stdio with a catalog introspected from the command tree, and `agent install --mcp` registers it in the three agents; `apply` is never a tool.
- A default project recorded with `gh board use`, a local audit log, plan and journal retention, and binaries for macOS, Linux and Windows on amd64 and arm64 with signed releases, checksums, SBOMs and artifact attestations.
