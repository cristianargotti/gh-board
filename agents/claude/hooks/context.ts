import type { ProcessRunResult } from "claude-code";

import type { BoardItem, BoardSnapshot, BoardSprint } from "../types";
import { isAlert, projectOf, recordOf, stringOf } from "./alerts";

export const CONTEXT_ARGV: readonly string[] = [
  "gh",
  "board",
  "context",
  "--json",
];

export const messageOf = (err: unknown): string =>
  err instanceof Error ? err.message : String(err);

/** The error text of a failed gh board context run, empty on success. */
export function contextError(ran: ProcessRunResult): string {
  if (ran.exitCode === 0) return "";
  return `gh board context exited ${ran.exitCode}: ${ran.stderr.trim() || ran.stdout.trim()}`;
}

export function parseSnapshot(text: string): BoardSnapshot | null {
  const record = recordOf(text);
  if (record === null || typeof record.generated_at !== "string") return null;
  return {
    generated_at: record.generated_at,
    project: projectOf(record.project),
    title: stringOf(record.title),
    viewer: stringOf(record.viewer),
    sprint: sprintOf(record.sprint),
    mine: itemsOf(record.mine),
    attention: Array.isArray(record.attention)
      ? record.attention.filter(isAlert)
      : [],
    triage: itemsOf(record.triage),
    completeness: record.completeness === "partial" ? "partial" : "complete",
    omitted: omittedOf(record.omitted),
  };
}

function sprintOf(value: unknown): BoardSprint | null {
  if (typeof value !== "object" || value === null) return null;
  const record = value as Record<string, unknown>;
  return {
    title: stringOf(record.title),
    start: stringOf(record.start),
    end: stringOf(record.end),
    days_left: numberOf(record.days_left),
    open: numberOf(record.open),
    done: numberOf(record.done),
  };
}

function itemsOf(value: unknown): BoardItem[] {
  if (!Array.isArray(value)) return [];
  return value.flatMap((entry) => {
    if (typeof entry !== "object" || entry === null) return [];
    const record = entry as Record<string, unknown>;
    const item: BoardItem = {
      ref: stringOf(record.ref),
      node_id: stringOf(record.node_id),
      title: stringOf(record.title),
      status: stringOf(record.status),
      assignees: Array.isArray(record.assignees)
        ? record.assignees.filter((a) => typeof a === "string")
        : [],
      url: stringOf(record.url),
    };
    return [withOptional(item, record)];
  });
}

function withOptional(
  item: BoardItem,
  record: Record<string, unknown>,
): BoardItem {
  const out = { ...item };
  for (const key of ["lane", "epic", "sprint", "target"] as const) {
    const value = record[key];
    if (typeof value === "string" && value !== "") out[key] = value;
  }
  return out;
}

function omittedOf(value: unknown): Record<string, number> {
  if (typeof value !== "object" || value === null) return {};
  const out: Record<string, number> = {};
  for (const [key, n] of Object.entries(value as Record<string, unknown>)) {
    if (typeof n === "number") out[key] = n;
  }
  return out;
}

const numberOf = (value: unknown): number =>
  typeof value === "number" ? value : 0;
