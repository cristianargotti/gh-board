import { expect, mock, test } from "claude-code/testing";
import type { ProcessRunResult } from "claude-code";
import type { On } from "claude-code";

const GUARD_ARGV = ["gh", "board", "guard", "check", "--agent", "claude"];
const DESTRUCTIVE = "gh project delete 999999 --owner nobody";

// The shell tools the guard covers: Bash everywhere, PowerShell on Windows
// where Claude Code may register no Bash tool at all.
const SHELL_TOOLS = ["Bash", "PowerShell"] as const;
type ShellTool = (typeof SHELL_TOOLS)[number];

type Answer = Partial<ProcessRunResult> & { exitCode: number };

const ran = (answer: Answer): ProcessRunResult => ({
  stdout: "",
  stderr: "",
  isStdoutTruncated: false,
  isStderrTruncated: false,
  ...answer,
});

// What the test holds beneath the plugin: the clock, the working directory,
// the kit's answer and the shell tool itself, which records whether it ran.
function stage(on: On, tool: ShellTool, answer: () => ProcessRunResult) {
  const calls: { argv: readonly string[]; stdin: string }[] = [];
  const executed: string[] = [];
  mock.clock(on);
  on("session.cwd", () => ({ value: "/work" }));
  on("process.run", ($, e) => {
    calls.push({ argv: e.argv, stdin: e.init?.stdin ?? "" });
    return { value: answer() };
  });
  on("tool.call", { tool }, ($, e) => {
    executed.push(e.command);
    return { result: { stdout: "ran", stderr: "", interrupted: false } };
  });
  return { calls, executed };
}

const deniedText = (result: {
  deny?: string;
  isError?: true;
  text?: string;
}): string | undefined =>
  result.deny ?? (result.isError ? result.text : undefined);

for (const tool of SHELL_TOOLS) {
  test(`${tool}: the guard asks the kit with the PreToolUse JSON and denies on exit 2`, async ($, on) => {
    const { calls, executed } = stage(on, tool, () =>
      ran({ exitCode: 2, stderr: "gh board guard: denied gh project delete" }),
    );
    const result = await $.tool.call({ tool, command: DESTRUCTIVE });
    expect(deniedText(result)).toContain("denied gh project delete");
    expect(executed).toEqual([]);
    expect(calls).toHaveLength(1);
    expect(calls[0]?.argv).toEqual(GUARD_ARGV);
    expect(JSON.parse(calls[0]?.stdin ?? "")).toMatchObject({
      hook_event_name: "PreToolUse",
      tool_name: tool,
      tool_input: { command: DESTRUCTIVE },
      cwd: "/work",
    });
  });

  test(`${tool}: the guard reads a JSON deny on exit 0 and lets other commands run`, async ($, on) => {
    const verdicts: ProcessRunResult[] = [
      ran({
        exitCode: 0,
        stdout: JSON.stringify({
          hookSpecificOutput: {
            hookEventName: "PreToolUse",
            permissionDecision: "deny",
            permissionDecisionReason: "destructive project command",
          },
        }),
      }),
      ran({ exitCode: 0 }),
    ];
    const { executed } = stage(
      on,
      tool,
      () => verdicts.shift() ?? ran({ exitCode: 0 }),
    );
    const denied = await $.tool.call({ tool, command: DESTRUCTIVE });
    expect(deniedText(denied)).toContain("destructive project command");
    const allowed = await $.tool.call({ tool, command: "gh board context" });
    expect(deniedText(allowed)).toBeUndefined();
    expect(executed).toEqual(["gh board context"]);
  });

  test(`${tool}: the guard fails closed when the kit cannot judge`, async ($, on) => {
    const { executed } = stage(on, tool, () =>
      ran({ exitCode: 1, stderr: "unknown command board" }),
    );
    const result = await $.tool.call({ tool, command: "ls" });
    expect(deniedText(result)).toContain("could not judge");
    expect(deniedText(result)).toContain("exit 1");
    expect(executed).toEqual([]);
  });

  test(`${tool}: the denial sent to the agent uses the English reason, not the team message`, async ($, on) => {
    const reason = "gh board guard: denied destructive project command";
    const { executed } = stage(on, tool, () =>
      ran({
        exitCode: 2,
        stderr: reason,
        stdout: JSON.stringify({
          systemMessage: "Comando bloqueado pela proteção dos agentes.",
          hookSpecificOutput: {
            hookEventName: "PreToolUse",
            permissionDecision: "deny",
            permissionDecisionReason: reason,
          },
        }),
      }),
    );
    const result = await $.tool.call({ tool, command: DESTRUCTIVE });
    expect(deniedText(result)).toBe(`gh-board: ${reason}`);
    expect(executed).toEqual([]);
  });

  test(`${tool}: the guard denies when the kit cannot even start`, async ($, on) => {
    const executed: string[] = [];
    mock.clock(on);
    on("session.cwd", () => ({ value: "/work" }));
    on("process.run", () => ({ deny: "gh is not installed" }));
    on("tool.call", { tool }, ($, e) => {
      executed.push(e.command);
      return { result: { stdout: "ran", stderr: "", interrupted: false } };
    });
    const result = await $.tool.call({ tool, command: "ls" });
    expect(deniedText(result)).toContain("could not judge");
    expect(executed).toEqual([]);
  });
}
