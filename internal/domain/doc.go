// Package domain holds the model and the rules of gh-board: projects, fields,
// items, references, the board.yml configuration, plans, alerts, snapshots,
// exit codes and the ports every adapter implements. It imports only the
// standard library so that every other package can depend on it (depguard
// enforces this boundary).
package domain
