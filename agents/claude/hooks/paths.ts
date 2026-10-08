export const STATE_FILE = "alerts.json";
const APP_DIR = "gh-board";
const GH_DIR = "gh";

/** The variables the state directory depends on, as the module read them. */
export type PathEnvironment = {
  GH_BOARD_HOME?: string;
  XDG_STATE_HOME?: string;
  OS?: string;
  LocalAppData?: string;
  HOME?: string;
  USERPROFILE?: string;
};

// Mirrors internal/config.Paths over go-gh's StateDir: GH_BOARD_HOME roots
// every directory (tests and CI), XDG_STATE_HOME wins on every operating
// system, LocalAppData on Windows, and ~/.local/state otherwise.
export function alertsPathFrom(env: PathEnvironment): string {
  if (isSet(env.GH_BOARD_HOME))
    return join(env.GH_BOARD_HOME, "state", STATE_FILE);
  if (isSet(env.XDG_STATE_HOME))
    return join(env.XDG_STATE_HOME, GH_DIR, APP_DIR, STATE_FILE);
  if (env.OS === "Windows_NT" && isSet(env.LocalAppData)) {
    return join(env.LocalAppData, "GitHub CLI", APP_DIR, STATE_FILE);
  }
  const user = isSet(env.HOME) ? env.HOME : (env.USERPROFILE ?? "");
  return join(user, ".local", "state", GH_DIR, APP_DIR, STATE_FILE);
}

function isSet(value: string | undefined): value is string {
  return value !== undefined && value !== "";
}

// A Windows base keeps its backslashes; everything else joins with a slash,
// which every file system call of the engine accepts.
function join(base: string, ...parts: string[]): string {
  const separator = base.includes("\\") ? "\\" : "/";
  const trimmed = base.replace(/[\\/]+$/, "");
  return [trimmed, ...parts].join(separator);
}
