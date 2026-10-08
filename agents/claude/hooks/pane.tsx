import type {
  BoxProps,
  ButtonProps,
  ElementConstructor,
  RenderElement,
  TextProps,
} from "claude-code";

import type {
  BoardAlert,
  BoardAlertState,
  BoardItem,
  BoardRefresh,
  BoardSnapshot,
  BoardWatch,
} from "../types";
import { INSTALL_WATCH, NO_WATCH, daysText } from "./alerts";

export const PANE_ID = "board";
export const PANE_TITLE = "Board";

// Rows per section; the pane is a glance, the verbs are the detail.
const MAX_ROWS = 8;
const MIN_WIDTH = 20;

const SEVERITY_MARK: Record<string, string> = {
  critical: "!!",
  warning: "!",
  info: "i",
};

export type PaneElements = {
  Box: ElementConstructor<BoxProps>;
  Text: ElementConstructor<TextProps>;
  Button: ElementConstructor<ButtonProps>;
};

export type PaneView = {
  snapshot: BoardSnapshot | null;
  alerts: BoardAlertState | null;
  watch: BoardWatch;
  refresh: BoardRefresh;
};

type Line = { text: string; isBold: boolean; isDim: boolean };

export function paneTree(
  elements: PaneElements,
  view: PaneView,
  columns: number,
  onRefresh: () => void,
): RenderElement {
  const { Box, Text, Button } = elements;
  const width = Math.max(MIN_WIDTH, columns);
  const label = view.refresh.isRunning ? "Atualizando..." : "Atualizar";
  const lines = sectionLines(view, width);
  return (
    <Box flexDirection="column">
      <Box gap={1}>
        <Text bold>
          {cut(headerText(view.snapshot), width - label.length - 3)}
        </Text>
        <Button key="refresh" hotkey="r" label={label} onPress={onRefresh} />
      </Box>
      {view.refresh.error !== "" && (
        <Text color="error">{cut(view.refresh.error, width)}</Text>
      )}
      {lines.map((line) => (
        <Text dimColor={line.isDim} bold={line.isBold}>
          {line.text}
        </Text>
      ))}
      <Text dimColor>{cut(footerText(view), width)}</Text>
    </Box>
  );
}

function sectionLines(view: PaneView, width: number): Line[] {
  return [
    ...section(
      "Meus itens",
      itemLines(view.snapshot?.mine ?? [], "nenhum item aberto"),
      width,
    ),
    ...section("Atenção", attentionLines(view), width),
    ...section(
      "Fila de triagem",
      itemLines(view.snapshot?.triage ?? [], "fila vazia"),
      width,
    ),
  ];
}

function section(title: string, lines: string[], width: number): Line[] {
  const shown = lines.slice(0, MAX_ROWS).map((text) => ({
    text: cut(`  ${text}`, width),
    isBold: false,
    isDim: false,
  }));
  const rest = lines.length - shown.length;
  const tail =
    rest > 0
      ? [{ text: `  ... e mais ${rest}`, isBold: false, isDim: true }]
      : [];
  return [{ text: title, isBold: true, isDim: false }, ...shown, ...tail];
}

export function headerText(snapshot: BoardSnapshot | null): string {
  if (snapshot === null) return "Board: pressione Atualizar para carregar";
  const sprint = snapshot.sprint;
  if (sprint === null) return `${snapshot.title}: sem sprint ativa`;
  return `${sprint.title} · ${daysText(sprint.days_left)} · ${sprint.open} abertas · ${sprint.done} concluídas`;
}

export function itemLines(items: BoardItem[], empty: string): string[] {
  if (items.length === 0) return [empty];
  return items.map((item) => `${item.ref} [${item.status}] ${item.title}`);
}

export function attentionLines(view: PaneView): string[] {
  const install = `${NO_WATCH}: ${INSTALL_WATCH}`;
  if (!view.watch.isInstalled && view.snapshot === null) return [install];
  const alerts = view.snapshot?.attention ?? view.alerts?.alerts ?? [];
  if (alerts.length === 0)
    return view.watch.isInstalled
      ? ["nada pendente"]
      : [`nada pendente · ${install}`];
  return alerts.map(alertLine);
}

const alertLine = (alert: BoardAlert): string => {
  const mark = SEVERITY_MARK[alert.severity] ?? "-";
  const ref =
    alert.item.number > 0
      ? ` (${alert.item.repository}#${alert.item.number})`
      : "";
  return `${mark} ${alert.message}${ref}`;
};

export function footerText(view: PaneView): string {
  const parts: string[] = [];
  if (view.snapshot !== null) {
    const omitted = Object.values(view.snapshot.omitted).reduce(
      (sum, n) => sum + n,
      0,
    );
    const cutNote =
      view.snapshot.completeness === "partial"
        ? ` · parcial, ${omitted} omitidos`
        : "";
    parts.push(`contexto de ${view.snapshot.generated_at}${cutNote}`);
  }
  if (view.alerts !== null)
    parts.push(`alertas de ${view.alerts.generated_at}`);
  return parts.length === 0 ? "sem dados ainda" : parts.join(" · ");
}

export function cut(text: string, width: number): string {
  const limit = Math.max(MIN_WIDTH, width);
  const chars = Array.from(text);
  return chars.length <= limit
    ? text
    : `${chars.slice(0, limit - 3).join("")}...`;
}
