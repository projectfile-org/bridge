// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package derive_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"kiota.ch/projectfile/core/v2/pkg/interp"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/derive"
)

func TestAddVirtualSplitsStableVersionOnly(t *testing.T) {
	const ladder = "${org.projectfile.semver.major}.${org.projectfile.semver.minor} ${org.projectfile.semver.major}"
	for version, want := range map[string]string{"0.8.3": "0.8 0", "12.4.0": "12.4 12", "0.9.0-rc.1": "", "": ""} {
		doc := &projectfile.Document{Identity: projectfile.Identity{Version: version}}
		derive.AddVirtual(doc)
		got, ok := interp.ExpandChecked(doc, ladder)
		if want == "" {
			assert.False(t, ok, version)
			continue
		}
		assert.Equal(t, want, got, version)
	}
}
