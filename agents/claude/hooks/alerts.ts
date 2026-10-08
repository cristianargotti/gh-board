import type {
  BoardAlert,
  BoardAlertState,
  BoardProject,
  BoardSnapshot,
  BoardWatch,
} from "../types";

/** Mirrors alerts.MaxToastsPerHour: what does not fit stays in the pane. */
export const MAX_TOASTS_PER_HOUR = 5;
export const HOUR_MS = 3_600_000;

export const NO_WATCH = "sem watch instalado";
export const INSTALL_WATCH = "gh board watch --install";
export const UNREADABLE = "alerts.json unreadable";

// The alert words of the status line, one row per rule id of section 9,
// singular then plural.
const RULE_WORDS: ReadonlyArray<readonly [string, string, string]> = [
  ["overdue", "atrasada", "atrasadas"],
  ["blocked", "bloqueada", "bloqueadas"],
  ["triage_sla", "triagem vencida", "triagens vencidas"],
  ["wip_exceeded", "WIP excedido", "WIP excedidos"],
  ["sprint_ending", "sprint acabando", "sprints acabando"],
  ["stale_active", "parada", "paradas"],
  ["epic_without_dates", "épico sem datas", "épicos sem datas"],
];

export const fingerprint = (alert: BoardAlert): string =>
  `${alert.rule_id}|${alert.item.node_id}`;

export function parseAlertState(text: string): BoardAlertState | null {
  const record = recordOf(text);
  if (record === null || !Array.isArray(record.alerts)) return null;
  return {
    generated_at: stringOf(record.generated_at),
    project: projectOf(record.project),
    summary: stringOf(record.summary),
    alerts: record.alerts.filter(isAlert),
  };
}

export function recordOf(text: string): Record<string, unknown> | null {
  let data: unknown;
  try {
    data = JSON.parse(text);
  } catch {
    return null;
  }
  return typeof data === "object" && data !== null
    ? (data as Record<string, unknown>)
    : null;
}

export const stringOf = (value: unknown): string =>
  typeof value === "string" ? value : "";

export function projectOf(value: unknown): BoardProject {
  const record =
    typeof value === "object" && value !== null
      ? (value as Record<string, unknown>)
      : {};
  return {
    owner: stringOf(record.owner),
    number: typeof record.number === "number" ? record.number : 0,
  };
}

export function isAlert(value: unknown): value is BoardAlert {
  if (typeof value !== "object" || value === null) return false;
  const record = value as Record<string, unknown>;
  const item = record.item;
  return (
    typeof record.rule_id === "string" &&
    typeof record.message === "string" &&
    typeof item === "object" &&
    item !== null &&
    typeof (item as Record<string, unknown>).node_id === "string"
  );
}

/** The alerts of current whose fingerprint previous did not hold. */
export function newAlerts(
  previous: BoardAlertState | null,
  current: BoardAlertState,
): BoardAlert[] {
  const seen = new Set((previous?.alerts ?? []).map(fingerprint));
  return current.alerts.filter((alert) => !seen.has(fingerprint(alert)));
}

/** How many toasts fit now, given when the last hour's were shown. */
export function toastRoom(
  times: number[],
  now: number,
): { recent: number[]; room: number } {
  const recent = times.filter((at) => now - at < HOUR_MS);
  return { recent, room: Math.max(0, MAX_TOASTS_PER_HOUR - recent.length) };
}

export function toastText(alert: BoardAlert): string {
  const ref =
    alert.item.number > 0
      ? ` (${alert.item.repository}#${alert.item.number})`
      : "";
  return `${alert.message}${ref}`;
}

export function statusText(
  snapshot: BoardSnapshot | null,
  state: BoardAlertState | null,
  watch: BoardWatch,
): string {
  const parts: string[] = [];
  const sprint = snapshot?.sprint ?? null;
  if (sprint !== null) parts.push(sprint.title, daysText(sprint.days_left));
  if (!watch.isInstalled) parts.push(NO_WATCH);
  else if (state === null)
    parts.push(watch.error === "" ? UNREADABLE : watch.error);
  else parts.push(summaryText(state));
  return parts.join(" · ");
}

export const daysText = (days: number): string =>
  days === 1 ? "1 dia" : `${days} dias`;

// The summary watch wrote wins; the counts are the fallback when it is empty.
export function summaryText(state: BoardAlertState): string {
  if (state.summary !== "") return state.summary;
  const counts = new Map<string, number>();
  for (const alert of state.alerts)
    counts.set(alert.rule_id, (counts.get(alert.rule_id) ?? 0) + 1);
  const parts = RULE_WORDS.flatMap(([rule, one, many]) => {
    const n = counts.get(rule) ?? 0;
    return n === 0 ? [] : [`${n} ${n === 1 ? one : many}`];
  });
  return parts.length === 0 ? "sem alertas" : parts.join(" · ");
}
