# Release checklist

Run this checklist on the integrated tree before tagging a release. Passing local checks is separate from publishing a release or testing a live agent session.

1. Run `make ci` on macOS, Linux and Windows. Keep the race tests, the 80 percent package and file coverage gates, the engineering laws, dependency checks and secret scans unchanged.
2. Build the binary with `go build -o /tmp/gh-board-docs ./cmd/gh-board` on macOS or Linux. Verify every documented command and flag against its help. Check `milestone create`, `milestone retarget`, `milestone set`, `roadmap`, `new`, `init` and `watch` explicitly. Run `go test ./` to check embedded assets and skill equality.
3. Run `claude plugin validate agents/claude` and `claude plugin test agents/claude`. Run both Codex policy checks in [agent-contracts.md](agent-contracts.md): deletion must be forbidden and listing must match no deny rule.
4. Run `make acceptance` (it runs `go run ./tools/acceptance`) as part of the release gate. The runner builds the binary, installs it as a temporary `gh` extension, runs `gh board agent install --agent all` into an isolated HOME, feeds `gh board guard check` the Claude Code, Cursor and Codex hook payloads for `gh project delete 999999 --owner nobody` and for `gh project list` (six checks: a denial with each agent's exit code and an allow), validates the generated Codex rules with `codex execpolicy check` and the plugin with `claude plugin validate` when those CLIs are on PATH, and prints a table; an absent CLI is reported as SKIP, never as a failure. The destructive input is never executed and the maintainer's real agent configuration is never touched. Keep this distinct from live agent acceptance: a hook payload proves the guard protocol, not that an installed agent loaded and trusted the hook.
5. Check init form rendering against the embedded PT-BR forms: configured names and labels substituted, board URL updated, unavailable issue types omitted, all form labels declared in `template.labels`, and no milestones invented. Decode the generated YAML strictly. `--output` must control where the draft and adjacent forms are written after apply.
6. Run `goreleaser check` and a local snapshot, `go run github.com/goreleaser/goreleaser/v2@v2.12.7 release --snapshot --skip=publish,sign --clean` (syft on PATH), and validate the release workflow. Confirm six binary targets, six SPDX SBOMs in `dist/`, checksums, signatures and artifact attestations: the SBOM catalog names the archive id `extension`, because the uploadable binaries of the binary archive format carry that id and a build id matches nothing (`tools/cicheck` checks it). Test installation from the candidate release in an isolated environment before publishing.
7. Record live agent acceptance separately for each supported agent and OS with a fake GitHub executable or another fixture that cannot mutate GitHub. Verify the hook is loaded and trusted, denial stops execution, reads remain available, and the five [assurance levels](security.md) still describe the measured behavior.

Do not run acceptance against real destructive endpoints, register a real scheduler, or change the maintainer's agent settings during build verification.

## Verifying a download

Every release carries, next to the six binaries, one SPDX JSON SBOM per binary, the `checksums.txt` manifest that covers the binaries and the SBOMs, the Sigstore bundle `checksums.txt.sigstore.json` that signs the manifest, and a GitHub artifact attestation per binary. Three checks work on the downloaded asset, from the strongest to the simplest; `<asset>` is the file name of the platform, for example `gh-board_0.1.1_darwin-arm64`.

1. The attestation binds the asset to the release workflow run of this repository. Use `--repo`, not `--owner`: the owner form queries the organizations API and fails for a user account.

   ```sh
   gh attestation verify <asset> --repo cristianargotti/gh-board
   ```

2. The Sigstore bundle proves that the manifest was signed by the release workflow (keyless, through the GitHub OIDC identity of the workflow).

   ```sh
   cosign verify-blob checksums.txt --bundle checksums.txt.sigstore.json \
     --certificate-identity-regexp '^https://github.com/cristianargotti/gh-board/' \
     --certificate-oidc-issuer https://token.actions.githubusercontent.com
   ```

3. The manifest lists the SHA-256 of every asset; compare it with the file you downloaded.

   ```sh
   grep <asset> checksums.txt | sha256sum --check   # shasum -a 256 --check on macOS
   ```

### The installed copy on Apple silicon

`gh extension install` runs `codesign --sign - --force` on the downloaded binary when the machine is an Apple silicon Mac (the file would not run otherwise), so the installed `gh-board` under the gh extensions directory carries an ad hoc signature with a new identifier, a different size and a different SHA-256 from the release asset. The code is the same: only the signature blob and the three header fields that size it change.

`gh board doctor` knows this. When the installed file differs from the release checksum it inspects the Mach-O code signature: a copy re-signed after the download is reported as a note, not a problem, and doctor downloads the release asset of the running version and platform once, checks it against the manifest (`Provenance: release asset matches the manifest`) and compares the installed copy with it apart from the signature (`Verified: yes`). A copy that differs beyond the signature, an asset that differs from the manifest, or a difference on a platform where gh does not re-sign, stays a problem. Run the three checks above on the downloaded asset, never on the installed copy.
