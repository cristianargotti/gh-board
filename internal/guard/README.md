# Guard layers

`patterns.json` is the policy source. The checker folds only executable
basenames and HTTP method values. Subcommands and GraphQL identifiers keep
their case. All 64 case spellings of the DELETE method are generated for
Claude's Bash and PowerShell rules and for expressible Codex prefixes.

Generated rules include `gh` and `gh.exe`. These rule languages have no
case-insensitive basename operator. Arbitrary executable casing, quoted or
unquoted Windows paths, and shell options in different orders are handled
by `guard check`. Strict mode also denies path-qualified gh reads and shell
command wrappers; normal mode inspects the inner script of a shell wrapper.
Normal mode retains the documented limits for path-qualified gh, env, eval,
and external file bodies. None of these layers is a general shell sandbox.

Codex prefixes cannot express methods after an arbitrary path, arbitrary
option order, GraphQL bodies, or shell script contents. The generated rules
state those limits. No shell executable is forbidden by a Codex prefix,
because that would also forbid the shell Codex uses to run ordinary reads.

Claude's [hooks reference](https://code.claude.com/docs/en/hooks) and
[tools reference](https://code.claude.com/docs/en/tools-reference) specify
`Bash|PowerShell` and the shared `tool_input.command` payload. Each scoped
Bash deny rule has a PowerShell counterpart so the installation does not
silently disable PowerShell on Windows with Git Bash.

`HookCommand` preserves the quoted POSIX form on Unix. On Windows it uses
`powershell.exe -NoProfile -NonInteractive -InputFormat Text -OutputFormat Text
-EncodedCommand <base64>`. Microsoft's
[PowerShell executable reference](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_powershell_exe?view=powershell-5.1)
specifies UTF-16LE for EncodedCommand and an explicit `exit $LASTEXITCODE`
to preserve the native exit status. The encoded payload protects spaces and
apostrophes in the resolved binary path from the outer shell. It forwards
the hook JSON on stdin with UTF-8 encoding and returns exit 2 on launch
failure. The outer command uses only plain arguments accepted by sh, Git
Bash and PowerShell, without cmd's MSYS path-conversion ambiguity or a
dependency on optional 8.3 filenames.

Tests exercise the decoded Windows payload, quoting, and sh/bash argument
roundtrips. Native Windows execution remains a Windows validation step.
