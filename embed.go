// Package assets embeds everything the installers need, because a
// precompiled gh extension ships only the executable (section 4.2): the
// skill, the agent hook fragments and mod sources under agents/, and the
// board.yml reference, issue forms and template README under templates/.
package assets

import "embed"

// FS holds agents/ and templates/ verbatim.
//
//go:embed all:agents all:templates
var FS embed.FS
