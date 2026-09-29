// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package buildinfo

import (
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromBuildInfo(t *testing.T) {
	devel := debug.Module{Version: "(devel)"}
	vcs := func(modified string) []debug.BuildSetting {
		return []debug.BuildSetting{{Key: "vcs.revision", Value: "406560c0123456789abc"}, {Key: "vcs.modified", Value: modified}}
	}
	cases := map[string]struct {
		bi   *debug.BuildInfo
		ok   bool
		want string
	}{
		"no build info":  {nil, false, "unknown"},
		"module version": {&debug.BuildInfo{Main: debug.Module{Version: "v2.10.0"}, Settings: vcs("true")}, true, "v2.10.0"},
		"clean revision": {&debug.BuildInfo{Main: devel, Settings: vcs("false")}, true, "406560c01234"},
		"dirty revision": {&debug.BuildInfo{Main: devel, Settings: vcs("true")}, true, "406560c01234-dirty"},
		"nothing usable": {&debug.BuildInfo{Main: devel}, true, "unknown"},
	}
	for name, c := range cases {
		assert.Equal(t, c.want, fromBuildInfo(c.bi, c.ok), name)
	}
}
