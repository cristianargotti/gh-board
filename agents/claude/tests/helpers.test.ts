import { describe, expect, test } from "claude-code/testing";

import { parseAlertState, statusText, summaryText } from "../hooks/alerts";
import { parseSnapshot } from "../hooks/context";
import { parseDecision, verdictOf } from "../hooks/guard";
import { alertsPathFrom } from "../hooks/paths";

const WATCH = {
  path: "/x/alerts.json",
  mtimeMs: 1,
  isInstalled: true,
  error: "",
};

const ran = (exitCode: number, stdout = "", stderr = "") => ({
  exitCode,
  stdout,
  stderr,
  isStdoutTruncated: false,
  isStderrTruncated: false,
});

const PATH_CASES = [
  {
    name: "GH_BOARD_HOME wins",
    env: { GH_BOARD_HOME: "/tmp/kit", HOME: "/home/dev" },
    want: "/tmp/kit/state/alerts.json",
  },
  {
    name: "XDG_STATE_HOME next",
    env: { XDG_STATE_HOME: "/var/state/", HOME: "/home/dev" },
    want: "/var/state/gh/gh-board/alerts.json",
  },
  {
    name: "LocalAppData on Windows",
    env: {
      OS: "Windows_NT",
      LocalAppData: "C:\\Users\\dev\\AppData\\Local",
      USERPROFILE: "C:\\Users\\dev",
    },
    want: "C:\\Users\\dev\\AppData\\Local\\GitHub CLI\\gh-board\\alerts.json",
  },
  {
    name: "home otherwise",
    env: { HOME: "/home/dev" },
    want: "/home/dev/.local/state/gh/gh-board/alerts.json",
  },
  {
    name: "USERPROFILE without HOME",
    env: { USERPROFILE: "C:\\Users\\dev" },
    want: "C:\\Users\\dev\\.local\\state\\gh\\gh-board\\alerts.json",
  },
];

const VERDICT_CASES = [
  { name: "exit 0 allows", ran: ran(0), want: undefined },
  {
    name: "exit 0 with a JSON deny denies",
    ran: ran(
      0,
      JSON.stringify({
        hookSpecificOutput: {
          permissionDecision: "deny",
          permissionDecisionReason: "no",
        },
      }),
    ),
    want: "no",
  },
  {
    name: "exit 0 with a JSON allow allows",
    ran: ran(
      0,
      JSON.stringify({ hookSpecificOutput: { permissionDecision: "allow" } }),
    ),
    want: undefined,
  },
  {
    name: "exit 2 denies with stderr",
    ran: ran(2, "", "denied: gh project delete\nmore"),
    want: "denied: gh project delete",
  },
  {
    name: "exit 2 without text denies",
    ran: ran(2),
    want: "denied by gh board guard",
  },
  {
    name: "exit 2 with the block shape denies",
    ran: ran(2, JSON.stringify({ decision: "block", reason: "blocked" })),
    want: "blocked",
  },
  {
    name: "exit 1 fails closed",
    ran: ran(1, "", "unknown command"),
    want: "gh board guard could not judge the command, so it was not run. Install or upgrade the gh-board extension and run gh board doctor (exit 1: unknown command)",
  },
];

describe("alertsPathFrom", () => {
  for (const c of PATH_CASES) {
    test(c.name, () => {
      expect(alertsPathFrom(c.env)).toBe(c.want);
    });
  }
});

describe("verdictOf", () => {
  for (const c of VERDICT_CASES) {
    test(c.name, () => {
      expect(verdictOf(c.ran)).toBe(c.want);
    });
  }
  test("parseDecision ignores text that is not JSON", () => {
    expect(parseDecision("not json")).toEqual({ isDenied: false });
  });
});

describe("alert state", () => {
  test("parseAlertState keeps only well formed alerts", () => {
    const state = parseAlertState(
      JSON.stringify({
        generated_at: "t",
        summary: "",
        alerts: [
          { rule_id: "overdue", message: "m", item: { node_id: "I_1" } },
          { rule_id: 1 },
        ],
      }),
    );
    expect(state?.alerts).toHaveLength(1);
    expect(parseAlertState("{")).toBeNull();
    expect(parseAlertState(JSON.stringify({ alerts: "x" }))).toBeNull();
  });
  test("summaryText prefers the summary watch wrote", () => {
    const state = parseAlertState(
      JSON.stringify({ summary: "2 atrasadas", alerts: [] }),
    );
    expect(state === null ? "" : summaryText(state)).toBe("2 atrasadas");
    const empty = parseAlertState(JSON.stringify({ summary: "", alerts: [] }));
    expect(empty === null ? "" : summaryText(empty)).toBe("sem alertas");
  });
  test("statusText names an unreadable file and a missing watch", () => {
    expect(statusText(null, null, { ...WATCH, isInstalled: false })).toBe(
      "sem watch instalado",
    );
    expect(statusText(null, null, WATCH)).toBe("alerts.json unreadable");
  });
});

describe("parseSnapshot", () => {
  test("accepts null sections and drops malformed entries", () => {
    const snapshot = parseSnapshot(
      JSON.stringify({
        generated_at: "t",
        sprint: null,
        mine: null,
        attention: null,
        triage: [
          { ref: "a/b#1", title: "x", status: "s", lane: "Frente", epic: "" },
          5,
        ],
        completeness: "partial",
        omitted: { mine: 2 },
      }),
    );
    expect(snapshot?.sprint).toBeNull();
    expect(snapshot?.mine).toEqual([]);
    expect(snapshot?.triage).toEqual([
      {
        ref: "a/b#1",
        node_id: "",
        title: "x",
        status: "s",
        assignees: [],
        url: "",
        lane: "Frente",
      },
    ]);
    expect(snapshot?.completeness).toBe("partial");
    expect(snapshot?.omitted).toEqual({ mine: 2 });
    expect(parseSnapshot("[]")).toBeNull();
  });
});
