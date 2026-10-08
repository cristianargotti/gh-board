import { expect, mock, test } from "claude-code/testing";
import type { Engine } from "claude-code/testing";
import type {
  CommandRunInput,
  On,
  ProcessRunResult,
  RenderPropsOf,
} from "claude-code";

const HOME = "/home/dev";
const ALERTS_PATH = `${HOME}/.local/state/gh/gh-board/alerts.json`;
const CONTEXT_ARGV = ["gh", "board", "context", "--json"];
const PANE_PROPS: RenderPropsOf["Pane"] = {
  title: "Board",
  isFocused: false,
  bodyColumns: 80,
  placement: "dock",
  scroll: { offset: 0, bodyRows: 30 },
  view: {},
};
const SURFACES = ["terminal", "desktop"] as const;
// /board as the person types it at a fullscreen terminal.
const BOARD_RUN: CommandRunInput = {
  command: "board",
  args: "",
  origin: { kind: "composer" },
  presentation: { isFullscreen: true, columns: 120 },
};

const alert = (rule: string, n: number, message: string) => ({
  rule_id: rule,
  severity: "warning",
  item: {
    node_id: `I_node${n}`,
    repository: "acme/sandbox",
    number: n,
    title: `Item ${n}`,
    url: `https://github.com/acme/sandbox/issues/${n}`,
  },
  message,
  first_seen: "2026-10-07T09:00:00Z",
});

const ALERTS = {
  generated_at: "2026-10-08T12:00:00Z",
  project: { owner: "acme", number: 2 },
  summary: "",
  alerts: [
    alert("overdue", 7, "Atrasada: Deploy"),
    alert("blocked", 8, "Bloqueada: Login"),
  ],
};

const SNAPSHOT = {
  generated_at: "2026-10-08T12:05:00Z",
  project: { owner: "acme", number: 2 },
  title: "Board de teste",
  viewer: "dev",
  sprint: {
    title: "Sprint 5",
    start: "2026-10-05T00:00:00Z",
    end: "2026-10-19T00:00:00Z",
    days_left: 3,
    open: 4,
    done: 2,
  },
  mine: [
    {
      ref: "acme/sandbox#7",
      node_id: "I_node7",
      title: "Deploy",
      status: "IN PROGRESS",
      assignees: ["dev"],
      url: "https://github.com/acme/sandbox/issues/7",
    },
  ],
  attention: ALERTS.alerts,
  triage: [
    {
      ref: "acme/sandbox#9",
      node_id: "I_node9",
      title: "Pedido novo",
      status: "BACKLOG",
      assignees: [],
      url: "https://github.com/acme/sandbox/issues/9",
    },
  ],
  epics: [],
  deliveries: [],
  completeness: "complete",
  omitted: {},
};

type Disk = { text: string | null; mtimeMs: number };

// The world beneath the plugin: a clock, a home, alerts.json on a fake disk,
// the context command, and the display calls recorded as they come.
function stage(on: On, disk: Disk) {
  const statuses: (string | undefined)[] = [];
  const toasts: string[] = [];
  const contextRuns: string[][] = [];
  const clock = mock.clock(on, { now: 1_000_000 });
  mock.env(on, { HOME });
  on("session.start", ($, e) => ({ cwd: e.cwd }));
  on("session.cwd", () => ({ value: "/work" }));
  on("command.register", ($, e) => ({ value: { command: e.name } }));
  on("ui.status", ($, e) => {
    statuses.push(e.text);
    return { value: undefined };
  });
  on("ui.toast", ($, e) => {
    toasts.push(e.text);
    return { value: undefined };
  });
  on("ui.open", () => ({ value: { isPlaced: true } }));
  on("fs.exists", ($, e) => {
    expect(e.path).toBe(ALERTS_PATH);
    return { value: disk.text !== null };
  });
  on("fs.stat", () => ({
    value: { kind: "file", size: 1, mtimeMs: disk.mtimeMs, isLink: false },
  }));
  on("fs.read", () => ({ value: disk.text ?? "" }));
  on("process.run", ($, e) => {
    contextRuns.push([...e.argv]);
    const ran: ProcessRunResult = {
      exitCode: 0,
      stdout: JSON.stringify(SNAPSHOT),
      stderr: "",
      isStdoutTruncated: false,
      isStderrTruncated: false,
    };
    return { value: ran };
  });
  return { clock, statuses, toasts, contextRuns };
}

const start = ($: Engine) =>
  $.session.start({ cwd: "/work", surface: "terminal", isInteractive: true });

test("without alerts.json the status line and the pane say sem watch instalado", async ($, on) => {
  const { statuses, contextRuns } = stage(on, { text: null, mtimeMs: 0 });
  await start($);
  expect(statuses.at(-1)).toBe("sem watch instalado");
  expect(contextRuns).toEqual([]);
  for (const surface of SURFACES) {
    const ui = await $.ui.mount({
      plugin: "gh-board",
      surface,
      component: "Pane",
      requestId: "board",
      props: PANE_PROPS,
    });
    const hint = await ui.find({ type: "Text", text: /sem watch instalado/ });
    expect(hint?.text).toContain("gh board watch --install");
    expect(await ui.find({ type: "Text", text: /Meus itens/ })).toBeDefined();
    await ui.unmount();
  }
});

test("the status line counts the cached alerts and /board adds the sprint", async ($, on) => {
  const { statuses, contextRuns, toasts } = stage(on, {
    text: JSON.stringify(ALERTS),
    mtimeMs: 1000,
  });
  await start($);
  expect(statuses.at(-1)).toBe("1 atrasada · 1 bloqueada");
  expect(toasts).toEqual([]);
  const answer = await $.command.run(BOARD_RUN);
  expect(answer.text).toBe("Board pane opened.");
  expect(contextRuns).toEqual([CONTEXT_ARGV]);
  expect(statuses.at(-1)).toBe("Sprint 5 · 3 dias · 1 atrasada · 1 bloqueada");
});

test("the pane draws the sprint, my items, attention and triage on every surface", async ($, on) => {
  const { contextRuns } = stage(on, {
    text: JSON.stringify(ALERTS),
    mtimeMs: 1000,
  });
  await start($);
  await $.command.run(BOARD_RUN);
  for (const surface of SURFACES) {
    const ui = await $.ui.mount({
      plugin: "gh-board",
      surface,
      component: "Pane",
      requestId: "board",
      props: PANE_PROPS,
    });
    const header = await ui.find({ type: "Text", text: /Sprint 5/ });
    expect(header?.text).toContain("3 dias");
    expect(
      await ui.find({
        type: "Text",
        text: /acme\/sandbox#7 \[IN PROGRESS\] Deploy/,
      }),
    ).toBeDefined();
    expect(
      await ui.find({ type: "Text", text: /Atrasada: Deploy/ }),
    ).toBeDefined();
    expect(
      await ui.find({
        type: "Text",
        text: /acme\/sandbox#9 \[BACKLOG\] Pedido novo/,
      }),
    ).toBeDefined();
    const runsBefore = contextRuns.length;
    await ui.press({ key: "refresh" });
    expect(contextRuns.length).toBe(runsBefore + 1);
    await ui.unmount();
  }
});

test("new alerts toast at most five per hour and the rest stay in the pane", async ($, on) => {
  const disk: Disk = { text: JSON.stringify(ALERTS), mtimeMs: 1000 };
  const { clock, toasts } = stage(on, disk);
  await start($);
  const many = Array.from({ length: 7 }, (_, i) =>
    alert("overdue", 20 + i, `Atrasada: tarefa ${20 + i}`),
  );
  disk.text = JSON.stringify({
    ...ALERTS,
    alerts: [...ALERTS.alerts, ...many],
  });
  disk.mtimeMs = 2000;
  await clock.advance(60_000);
  expect(toasts).toHaveLength(5);
  expect(toasts[0]).toBe("Atrasada: tarefa 20 (acme/sandbox#20)");
  disk.text = JSON.stringify({
    ...ALERTS,
    alerts: [
      ...ALERTS.alerts,
      ...many,
      alert("blocked", 30, "Bloqueada: tarefa 30"),
    ],
  });
  disk.mtimeMs = 3000;
  await clock.advance(60_000);
  expect(toasts).toHaveLength(5);
  // What the cap held back is still in the pane's attention list.
  const ui = await $.ui.mount({
    plugin: "gh-board",
    surface: "terminal",
    component: "Pane",
    requestId: "board",
    props: PANE_PROPS,
  });
  expect(await ui.find({ type: "Text", text: /tarefa 25/ })).toBeDefined();
  expect(await ui.find({ type: "Text", text: /e mais 2/ })).toBeDefined();
  await ui.unmount();
  // Once the hour has passed the next new alert toasts again.
  await clock.advance(3_600_000);
  disk.text = JSON.stringify({
    ...ALERTS,
    alerts: [...ALERTS.alerts, alert("blocked", 31, "Bloqueada: tarefa 31")],
  });
  disk.mtimeMs = 4000;
  await clock.advance(60_000);
  expect(toasts).toHaveLength(6);
  expect(toasts.at(-1)).toBe("Bloqueada: tarefa 31 (acme/sandbox#31)");
});
