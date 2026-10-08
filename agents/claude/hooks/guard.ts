import type { ProcessRunResult } from "claude-code";

export const GUARD_ARGV: readonly string[] = [
  "gh",
  "board",
  "guard",
  "check",
  "--agent",
  "claude",
];
const DENIED = "denied by gh board guard";

export const GUARD_UNAVAILABLE =
  "gh board guard could not judge the command, so it was not run. Install or upgrade the gh-board extension and run gh board doctor";

type Decision = {
  isDenied: boolean;
  reason?: string;
};

/** The JSON a PreToolUse command hook receives, which the kit reads on stdin. */
export function guardInput(
  command: string,
  toolUseId: string | undefined,
  cwd: string,
  tool: "Bash" | "PowerShell" = "Bash",
): string {
  return JSON.stringify({
    hook_event_name: "PreToolUse",
    tool_name: tool,
    tool_input: { command },
    tool_use_id: toolUseId ?? "",
    session_id: "",
    transcript_path: "",
    cwd,
  });
}

// Exit 2 denies, exit 0 allows unless the JSON says deny, and any other
// exit denies too: a guard that cannot judge fails closed.
export function verdictOf(ran: ProcessRunResult): string | undefined {
  const decision = parseDecision(ran.stdout);
  const text = textOf(ran);
  if (ran.exitCode === 2)
    return decision.reason ?? (text === "" ? DENIED : text);
  if (ran.exitCode !== 0)
    return `${GUARD_UNAVAILABLE} (exit ${ran.exitCode}${text === "" ? "" : `: ${text}`})`;
  return decision.isDenied ? (decision.reason ?? DENIED) : undefined;
}

export function parseDecision(stdout: string): Decision {
  let data: unknown;
  try {
    data = JSON.parse(stdout);
  } catch {
    return { isDenied: false };
  }
  if (typeof data !== "object" || data === null) return { isDenied: false };
  const record = data as Record<string, unknown>;
  const specific = record.hookSpecificOutput;
  if (typeof specific === "object" && specific !== null) {
    const out = specific as Record<string, unknown>;
    const isDenied = out.permissionDecision === "deny";
    return isDenied
      ? { isDenied, reason: reasonOf(out.permissionDecisionReason) }
      : { isDenied };
  }
  if (record.decision === "block")
    return { isDenied: true, reason: reasonOf(record.reason) };
  return { isDenied: false };
}

const reasonOf = (value: unknown): string | undefined =>
  typeof value === "string" && value !== "" ? value : undefined;

function textOf(ran: ProcessRunResult): string {
  const err = ran.stderr.trim();
  return err !== "" ? firstLine(err) : firstLine(ran.stdout.trim());
}

const firstLine = (text: string): string => text.split("\n")[0] ?? "";
