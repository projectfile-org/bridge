// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package describe

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

const (
	probeShort = "annotate each sibling without loading it"
	notProbe   = "--described"
)

func TestRequested(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "nil args", args: nil, want: false},
		{name: "no probe", args: []string{"run", "cff"}, want: false},
		{name: "lone probe", args: []string{Flag}, want: true},
		{name: "probe after a verb", args: []string{"to", "all", Flag}, want: true},
		{name: "prefix is not the probe", args: []string{notProbe}, want: false},
		{name: "longer flag", args: []string{"--describe-all"}, want: false},
		{name: "repeated probe", args: []string{Flag, Flag}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Requested(tt.args), tt.name)
		})
	}
}

func TestHandledNotRequested(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{Short: probeShort}
	cmd.SetOut(&out)
	assert.False(t, Handled(cmd, []string{"run", "cff"}), "args carry no probe")
	assert.Empty(t, out.String(), "a command with no probe must print nothing")
}

func TestHandledPrintsShort(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{Short: probeShort}
	cmd.SetOut(&out)
	assert.True(t, Handled(cmd, []string{Flag}), "the probe is answered")
	assert.Equal(t, probeShort+"\n", out.String(), "one description line on stdout")
}

func TestHandledEmptyShortStillAnswered(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	assert.True(t, Handled(cmd, []string{Flag}), "the probe is answered even with no Short")
	assert.Equal(t, "\n", out.String(), "the empty Short still terminates its line")
}
