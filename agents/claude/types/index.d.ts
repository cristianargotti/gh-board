// The $.state contract of the gh-board plugin: what the mod keeps for the
// session so that a hot reload of its code loses nothing. BoardAlertState
// mirrors alerts.json as gh board watch writes it (domain.AlertState) and
// BoardSnapshot mirrors gh board context --json (domain.Snapshot), reduced
// to the sections the pane draws. Field names are the JSON names of the Go
// types, so a change there is a change here.

export type BoardProject = {
  owner: string;
  number: number;
};

export type BoardAlertItem = {
  node_id: string;
  project_item_id?: string;
  repository: string;
  number: number;
  title: string;
  url: string;
};

export type BoardSeverity = "info" | "warning" | "critical";

export type BoardAlert = {
  rule_id: string;
  severity: BoardSeverity;
  item: BoardAlertItem;
  message: string;
  first_seen: string;
};

export type BoardAlertState = {
  generated_at: string;
  project: BoardProject;
  summary: string;
  alerts: BoardAlert[];
};

export type BoardSprint = {
  title: string;
  start: string;
  end: string;
  days_left: number;
  open: number;
  done: number;
};

export type BoardItem = {
  ref: string;
  node_id: string;
  title: string;
  status: string;
  assignees: string[];
  lane?: string;
  epic?: string;
  sprint?: string;
  target?: string;
  url: string;
};

export type BoardSnapshot = {
  generated_at: string;
  project: BoardProject;
  title: string;
  viewer: string;
  sprint: BoardSprint | null;
  mine: BoardItem[];
  attention: BoardAlert[];
  triage: BoardItem[];
  completeness: "complete" | "partial";
  omitted: Record<string, number>;
};

/** Where alerts.json is and what the mod last saw of it. */
export type BoardWatch = {
  path: string;
  mtimeMs: number;
  isInstalled: boolean;
  error: string;
};

/** The last run of gh board context --json. */
export type BoardRefresh = {
  isRunning: boolean;
  error: string;
  at: number;
};

declare module "claude-code" {
  interface PluginState {
    "gh-board": {
      alerts: BoardAlertState | null;
      snapshot: BoardSnapshot | null;
      watch: BoardWatch;
      refresh: BoardRefresh;
      /** When each toast of the last hour was shown, for the cap of five. */
      toastTimes: number[];
    };
  }
}
