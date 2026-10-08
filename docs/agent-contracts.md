# Agent integration contracts

Validated locally with Codex CLI 0.159.3 and Claude Code 2.1.294. The shared skill source is `agents/SKILL.md`; after editing it, copy it verbatim to `agents/claude/skills/gh-board/SKILL.md`. `go test ./` checks equality and the complete embedded installer asset tree, including hidden plugin metadata.

## Codex

The kit targets `~/.codex/hooks.json`. Codex also reads trusted project `.codex/hooks.json` files next to active config layers. The fragment has this shape:

```json
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "^Bash$",
      "hooks": [{
        "type": "command",
        "command": "gh board guard check --agent codex",
        "timeout": 10
      }]
    }]
  }
}
```

`matcher` is a regex over tool names and aliases; `Bash` covers shell and `exec_command`. Omitting it, using an empty string, or `*` matches every tool. Anchors prevent matching similarly named MCP tools. Changed hooks require trust review in `/hooks`. [Codex hooks reference](https://developers.openai.com/codex/hooks).

A deny can return the hook-specific JSON below, or exit 2 with the reason on stderr. Other failures can let the tool continue. [Codex denial contract](https://developers.openai.com/codex/hooks#pretooluse).

Rules live at `~/.codex/rules/gh-board.rules`. `prefix_rule(pattern=["gh", "project", "delete"], decision="forbidden", justification="Use gh board verbs")` is valid Starlark; tokens match an exact argument prefix, and the strongest matching decision wins. [Codex rules reference](https://developers.openai.com/codex/rules).

```sh
codex execpolicy check --rules agents/codex/gh-board.rules gh project delete 999999 --owner nobody
codex execpolicy check --rules agents/codex/gh-board.rules gh project list
```

The first returns `decision: forbidden`; the second returns `matchedRules: []`. Both checks exit 0 because evaluating a policy is successful even when the command is forbidden. No match means this kit adds no restriction, not an explicit allow overriding other policies. Neither check executes GitHub commands.

## Claude Code

The plugin's `hooks/hooks.json` declares `hooks.PreToolUse`, a `matcher: "Bash"` group, and a command handler. It also registers the local mod module. A command hook receives `tool_name` and `tool_input.command` on stdin. On exit 0 it can deny with:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "gh board guard: denied destructive project command"
  }
}
```

Alternatively, exit 2 blocks and passes stderr to the agent; stdout JSON is ignored on that path. The kit emits the structured answer and exits 2 with the English reason on stderr. The mod reads `permissionDecisionReason` and sends an English `tool.call` denial, keeping the PT-BR `systemMessage` for the team. [Claude Code hooks reference](https://code.claude.com/docs/en/hooks#pretooluse-decision-control).

```sh
claude plugin validate agents/claude
claude plugin test agents/claude
```

These checks validate the package and exercise the mod with mocked tools. They do not install it or prove enforcement in a live agent session. Installation uses `claude plugin marketplace add cristianargotti/gh-board` followed by `claude plugin install gh-board@gh-board` as described in [setup.md](setup.md).
