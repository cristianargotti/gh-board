package commands

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

var commandCatalog = []struct {
	path, group, flags string
}{
	{"context", GroupRead, "budget for"},
	{"status", GroupRead, ""},
	{"me", GroupRead, ""},
	{"item", GroupRead, ""},
	{"list", GroupRead, "status assignee lane epic sprint label overdue blocked triage type all"},
	{"epics", GroupRead, ""},
	{"roadmap", GroupRead, "months"},
	{"sprint", GroupRead, ""},
	{"sprint current", GroupRead, ""},
	{"sprint next", GroupRead, ""},
	{"attention", GroupRead, ""},
	{"search", GroupRead, ""},
	{"schema", GroupRead, "refresh"},
	{"doctor", GroupRead, ""},
	{"log", GroupRead, "since"},
	{"version", GroupRead, ""},
	{"standup", GroupRead, "for"},
	{"digest", GroupRead, "week"},
	{"digest print", GroupRead, "week"},
	{"digest post", GroupPlan, "status"},
	{"plan", GroupRead, ""},
	{"plan list", GroupRead, ""},
	{"plan show", GroupRead, ""},
	{"new", GroupWrite, "title body parent lane epic sprint estimate start target milestone assignee label"},
	{"move", GroupWrite, ""},
	{"assign", GroupWrite, ""},
	{"unassign", GroupWrite, ""},
	{"set", GroupWrite, ""},
	{"sprint set", GroupWrite, ""},
	{"estimate", GroupWrite, ""},
	{"dates", GroupWrite, "start target"},
	{"milestone", GroupPlan, ""},
	{"milestone set", GroupWrite, ""},
	{"comment", GroupWrite, ""},
	{"link", GroupWrite, "parent"},
	{"label", GroupWrite, ""},
	{"reopen", GroupWrite, ""},
	{"restore", GroupWrite, ""},
	{"close", GroupPlan, ""},
	{"tidy", GroupPlan, ""},
	{"init", GroupPlan, "from forms owner repository title output"},
	{"milestone create", GroupPlan, "due description"},
	{"milestone retarget", GroupPlan, ""},
	{"apply", GroupPlan, ""},
	{"use", GroupAuto, "clear"},
	{"watch", GroupAuto, "once install interval uninstall"},
	{"agent", GroupAuto, ""},
	{"agent install", GroupAuto, "agent strict scope mcp"},
	{"agent uninstall", GroupAuto, "agent strict scope mcp"},
	{"guard", GroupAuto, ""},
	{"guard check", GroupAuto, "agent strict"},
	{"guard install", GroupAuto, "agent strict scope"},
	{"guard uninstall", GroupAuto, "agent strict scope"},
	{"mcp", GroupAuto, ""},
}

var commandTiers = []func(*cobra.Command, *Deps){
	registerReadCommands, registerWriteCommands, registerPlanCommands, registerAutoCommands,
}

func TestCommandRegistrationOrders(t *testing.T) {
	for _, order := range commandOrders(nil, []int{0, 1, 2, 3}) {
		t.Run(fmt.Sprint(order), func(t *testing.T) {
			deps := &Deps{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
			root := &cobra.Command{Use: "board"}
			bindGlobalFlags(root, &deps.Flags)
			for _, tier := range order {
				commandTiers[tier](root, deps)
			}
			assertCommandCatalog(t, root)
			assertCommandHelp(t, root)
			root.SetOut(deps.Out)
			root.SetArgs([]string{"--help"})
			if err := root.ExecuteContext(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func commandOrders(prefix, remaining []int) [][]int {
	if len(remaining) == 0 {
		return [][]int{prefix}
	}
	var orders [][]int
	for i, tier := range remaining {
		next := append(append([]int(nil), remaining[:i]...), remaining[i+1:]...)
		order := append(append([]int(nil), prefix...), tier)
		orders = append(orders, commandOrders(order, next)...)
	}
	return orders
}

func commandTree(t *testing.T, parent *cobra.Command, prefix string, found map[string]*cobra.Command) {
	t.Helper()
	for _, cmd := range parent.Commands() {
		path := strings.TrimSpace(prefix + " " + cmd.Name())
		if found[path] != nil {
			t.Fatalf("duplicate command %q", path)
		}
		found[path] = cmd
		commandTree(t, cmd, path, found)
	}
}

func assertCommandCatalog(t *testing.T, root *cobra.Command) {
	t.Helper()
	found := map[string]*cobra.Command{}
	commandTree(t, root, "", found)
	if len(found) != len(commandCatalog) {
		t.Fatalf("commands = %d, want %d: %v", len(found), len(commandCatalog), found)
	}
	for _, entry := range commandCatalog {
		cmd := found[entry.path]
		if cmd == nil || cmd.GroupID != entry.group {
			t.Fatalf("missing or incorrectly grouped command %q: %+v", entry.path, cmd)
		}
		for _, flag := range strings.Fields(entry.flags) {
			if cmd.Flags().Lookup(flag) == nil {
				t.Errorf("%s missing --%s", entry.path, flag)
			}
		}
		for _, flag := range strings.Fields("project config json format dry-run reason expect") {
			if cmd.InheritedFlags().Lookup(flag) == nil {
				t.Errorf("%s missing global --%s", entry.path, flag)
			}
		}
	}
}

func assertCommandHelp(t *testing.T, cmd *cobra.Command) {
	t.Helper()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.Help(); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, line := range strings.Split(out.String(), "\n") {
		if fields := strings.Fields(line); len(fields) > 0 && strings.HasPrefix(line, "  ") {
			counts[fields[0]]++
		}
	}
	for _, child := range cmd.Commands() {
		if counts[child.Name()] != 1 {
			t.Fatalf("%s help lists %s %d times:\n%s", cmd.CommandPath(), child.Name(), counts[child.Name()], out)
		}
		assertCommandHelp(t, child)
	}
}
