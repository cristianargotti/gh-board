package guard

import (
	"fmt"
	"strings"
	"unicode"
)

// clipLimit bounds the command text quoted back to the agent and the
// person; the command is external text and is shown as data.
const clipLimit = 120

// familyLabels names each family for the agent (English) and the team
// (PT-BR).
var familyLabels = map[string][2]string{
	"project": {"project write", "escrita em projetos"},
	"cli":     {"destructive CLI", "CLI destrutivo"},
	"api":     {"API mutation", "mutacao de API"},
	"strict":  {"strict mode", "modo estrito"},
}

func familyLabel(family string, pt bool) string {
	labels, ok := familyLabels[family]
	if !ok {
		return family
	}
	if pt {
		return labels[1]
	}
	return labels[0]
}

// agentMessage is the English reason the agent receives.
func agentMessage(m Match) string {
	return fmt.Sprintf(
		"gh board guard: denied '%s' because it matches the %s rule '%s'. "+
			"This command is destructive or reserved to humans. Use the gh board verbs instead "+
			"(context, status, item, list, move, assign, set, comment, new; close and tidy write a plan) "+
			"and hand plans to the person, who runs gh board apply <id> in a terminal.",
		clip(m.Segment), familyLabel(m.Family, false), m.Rule)
}

// humanMessage is the PT-BR reason the person sees.
func humanMessage(m Match) string {
	return fmt.Sprintf(
		"gh board: comando bloqueado pela proteção dos agentes. '%s' corresponde à regra '%s' (%s). "+
			"Operações destrutivas ou reservadas a humanos não são executadas por agentes: "+
			"use os verbos do gh board ou execute o comando você mesmo no terminal.",
		clip(m.Segment), m.Rule, familyLabel(m.Family, true))
}

// clip replaces control characters with spaces and truncates to clipLimit
// runes, marking the cut with "...".
func clip(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n == clipLimit {
			b.WriteString("...")
			break
		}
		if unicode.IsControl(r) {
			r = ' '
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}
