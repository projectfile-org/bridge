// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package describe owns the one-line self-introduction the pf-bridge
// dispatcher probes its children for: `pf-bridge-<name> --describe` prints a
// single description line so `pf-bridge --list` can annotate every installed
// sibling without the dispatcher knowing anything about bridges. The flag
// spelling lives here so dispatcher and children can never drift apart.
package describe

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"
)

// Flag is the probe, spelled as typed on the command line.
const Flag = "--describe"

// Requested reports whether args carry the dispatcher probe.
func Requested(args []string) bool {
	return slices.Contains(args, Flag)
}

// Handled answers the probe with cmd's Short and reports true, so a Main
// returns before any real work. Convenience for commands whose Short already
// says it all.
func Handled(cmd *cobra.Command, args []string) bool {
	if !Requested(args) {
		return false
	}
	fmt.Fprintln(cmd.OutOrStdout(), cmd.Short)
	return true
}
