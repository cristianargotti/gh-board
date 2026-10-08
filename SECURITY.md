# Security policy

## Supported versions

The latest release on the main branch receives fixes. Older releases are not patched; upgrade with `gh extension upgrade gh-board`.

## Reporting a vulnerability

Use GitHub private vulnerability reporting on this repository (Security tab, "Report a vulnerability"). Do not open a public issue for a security problem. Reports are acknowledged within five business days.

## What the kit protects and what it cannot

The security model, the mutation allowlist, the guard layers in the agents and the residual risks are documented in `docs/security.md`. A developer token that GitHub lets delete things can still delete things through channels the kit does not see; the kit removes those paths from its own binary and from the agents it guards, and makes consequential changes a human action through plans.
