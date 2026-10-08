import { atom, read, update } from "claude-code";
import type { EngineInterface, ProcessRunResult, Register } from "claude-code";

import type { BoardAlertState } from "../types";
import {
  UNREADABLE,
  newAlerts,
  parseAlertState,
  statusText,
  toastRoom,
  toastText,
} from "./alerts";
import {
  CONTEXT_ARGV,
  contextError,
  messageOf,
  parseSnapshot,
} from "./context";
import { GUARD_ARGV, GUARD_UNAVAILABLE, guardInput, verdictOf } from "./guard";
import { PANE_ID, PANE_TITLE, paneTree } from "./pane";
import type { PaneView } from "./pane";
import { alertsPathFrom } from "./paths";

// How often the mod stats alerts.json: a local file, never GitHub, which
// watch polls on its own schedule (ten minutes by default).
const POLL_MS = 60_000;
const CONTEXT_TIMEOUT_MS = 60_000;
const GUARD_TIMEOUT_MS = 10_000;

// One atom per key of the contract, declared here because the engine reads
// the state a module touches off the hooks module's own source.
const alertsAtom = atom({ plugin: "gh-board", key: "alerts" } as const, null);
const snapshotAtom = atom(
  { plugin: "gh-board", key: "snapshot" } as const,
  null,
);
const watchAtom = atom({ plugin: "gh-board", key: "watch" } as const, {
  path: "",
  mtimeMs: 0,
  isInstalled: false,
  error: "",
});
const refreshAtom = atom({ plugin: "gh-board", key: "refresh" } as const, {
  isRunning: false,
  error: "",
  at: 0,
});
const toastTimesAtom = atom(
  { plugin: "gh-board", key: "toastTimes" } as const,
  [],
);

type Loaded = {
  state: BoardAlertState | null;
  mtimeMs: number;
  isInstalled: boolean;
};

// Every function that takes $ lives in this file: the engine follows $ only
// inside the hooks module, never across an import.

async function resolveAlertsPath($: EngineInterface): Promise<string> {
  return alertsPathFrom({
    GH_BOARD_HOME: await $.env.get("GH_BOARD_HOME"),
    XDG_STATE_HOME: await $.env.get("XDG_STATE_HOME"),
    OS: await $.env.get("OS"),
    LocalAppData: await $.env.get("LocalAppData"),
    HOME: await $.env.get("HOME"),
    USERPROFILE: await $.env.get("USERPROFILE"),
  });
}

async function loadAlertFile(
  $: EngineInterface,
  path: string,
): Promise<Loaded> {
  if (!(await $.fs.exists(path)))
    return { state: null, mtimeMs: 0, isInstalled: false };
  const stat = await $.fs.stat(path);
  const text = await $.fs.read(path);
  const state = parseAlertState(typeof text === "string" ? text : "");
  return { state, mtimeMs: stat.mtimeMs, isInstalled: true };
}

// Reads alerts.json again only when it changed since the last look, toasts
// what is new when asked, and keeps the status line current.
async function syncAlerts($: EngineInterface, toast: boolean): Promise<void> {
  const watch = await read($, watchAtom);
  if (watch.path === "") return;
  const loaded = await loadAlertFile($, watch.path);
  if (
    loaded.isInstalled === watch.isInstalled &&
    loaded.mtimeMs === watch.mtimeMs
  )
    return;
  const previous = await read($, alertsAtom);
  await update($, alertsAtom, () => loaded.state);
  const error = loaded.isInstalled && loaded.state === null ? UNREADABLE : "";
  await update($, watchAtom, (w) => ({
    ...w,
    mtimeMs: loaded.mtimeMs,
    isInstalled: loaded.isInstalled,
    error,
  }));
  if (toast && loaded.state !== null) await toastNew($, previous, loaded.state);
  await showStatus($);
}

async function toastNew(
  $: EngineInterface,
  previous: BoardAlertState | null,
  current: BoardAlertState,
): Promise<void> {
  const fresh = newAlerts(previous, current);
  if (fresh.length === 0) return;
  const now = await $.clock.now();
  const { recent, room } = toastRoom(await read($, toastTimesAtom), now);
  const shown = fresh.slice(0, room);
  for (const alert of shown) $.ui.toast(toastText(alert));
  await update($, toastTimesAtom, () => [...recent, ...shown.map(() => now)]);
}

async function showStatus($: EngineInterface): Promise<void> {
  const snapshot = await read($, snapshotAtom);
  const state = await read($, alertsAtom);
  const watch = await read($, watchAtom);
  $.ui.status(statusText(snapshot, state, watch));
}

// One run at a time: the button and /board may both ask while one is going.
async function refreshSnapshot($: EngineInterface): Promise<void> {
  if ((await read($, refreshAtom)).isRunning) return;
  await update($, refreshAtom, (r) => ({ ...r, isRunning: true }));
  const error = await runContext($);
  const at = await $.clock.now();
  await update($, refreshAtom, () => ({ isRunning: false, error, at }));
  await syncAlerts($, true);
  await showStatus($);
}

async function runContext($: EngineInterface): Promise<string> {
  let ran: ProcessRunResult;
  try {
    ran = await $.process.run(CONTEXT_ARGV, { timeoutMs: CONTEXT_TIMEOUT_MS });
  } catch (err) {
    return `gh board context could not run: ${messageOf(err)}`;
  }
  const error = contextError(ran);
  if (error !== "") return error;
  const snapshot = parseSnapshot(ran.stdout);
  if (snapshot === null) return "gh board context returned no JSON snapshot";
  await update($, snapshotAtom, () => snapshot);
  return "";
}

// Asks the kit with the same JSON a PreToolUse command hook receives.
async function guardVerdict(
  $: EngineInterface,
  command: string,
  toolUseId: string | undefined,
  tool: "Bash" | "PowerShell",
): Promise<string | undefined> {
  const stdin = guardInput(command, toolUseId, await $.session.cwd(), tool);
  const ran = await $.process.run(GUARD_ARGV, {
    stdin,
    timeoutMs: GUARD_TIMEOUT_MS,
  });
  return verdictOf(ran);
}

// The baseline load at session start toasts nothing: the person has not
// seen this session's alerts yet, and the pane lists them all.
async function startSession($: EngineInterface): Promise<void> {
  await $.command.register({
    name: "board",
    description:
      "Open the board pane: sprint and days left, my items, attention and triage queue",
  });
  const path = await resolveAlertsPath($);
  await update($, watchAtom, (watch) => ({ ...watch, path }));
  await syncAlerts($, false);
  await showStatus($);
  $.clock.every(POLL_MS, () => void syncAlerts($, true));
}

async function openBoard($: EngineInterface): Promise<string> {
  const opened = await $.ui.open({ id: PANE_ID, title: PANE_TITLE });
  await refreshSnapshot($);
  return opened.isPlaced
    ? "Board pane opened."
    : `Board pane is waiting: ${opened.reason}`;
}

async function viewOf($: EngineInterface): Promise<PaneView> {
  return {
    snapshot: await read($, snapshotAtom),
    alerts: await read($, alertsAtom),
    watch: await read($, watchAtom),
    refresh: await read($, refreshAtom),
  };
}

export const register: Register = (on) => {
  on("session.start", async ($, e, next) => {
    await startSession($);
    return next(e);
  });

  on("command.run", { command: "board" }, async ($) => ({
    text: await openBoard($),
  }));

  on("ui.render", { component: "Pane", requestId: "board" }, async ($, e) =>
    paneTree($.ui.resolve(e), await viewOf($), e.props.bodyColumns, () => {
      void refreshSnapshot($);
    }),
  );

  on("tool.call", { tool: ["Bash", "PowerShell"] }, async ($, e, next) => {
    const deny = await guardVerdict($, e.command, e.tool_use_id, e.tool);
    return deny === undefined ? next(e) : { deny: `${$.plugin.name}: ${deny}` };
  }).catch(($, e, next) =>
    next.called ? next(e) : { deny: `${$.plugin.name}: ${GUARD_UNAVAILABLE}` },
  );
};
