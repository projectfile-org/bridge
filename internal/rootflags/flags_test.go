// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package rootflags

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestVersionShortAndLongFlags(t *testing.T) {
	for _, arg := range []string{"-V", "--version"} {
		root := &cobra.Command{Use: "pf-bridge-test", Version: "1.2.3", Run: func(*cobra.Command, []string) {}}
		Bind(root)
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs([]string{arg})
		assert.NoError(t, root.Execute(), arg)
		assert.Equal(t, "pf-bridge-test version 1.2.3\n", out.String(), arg)
	}
}

func TestVerboseKeepsShortV(t *testing.T) {
	root := &cobra.Command{Use: "pf-bridge-test", Version: "1.2.3", Run: func(*cobra.Command, []string) {}}
	Bind(root)
	assert.Equal(t, "verbose", root.PersistentFlags().ShorthandLookup("v").Name)
	assert.Equal(t, "version", root.Flags().ShorthandLookup("V").Name)
}
