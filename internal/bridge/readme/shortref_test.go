// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShortRefDropsDockerHubHost(t *testing.T) {
	assert.Equal(t, "docker pull damianbuho/x:latest", shortRef("docker pull docker.io/damianbuho/x:latest"))
	assert.Equal(t, "damianbuho/x:latest", shortRef("docker.io/damianbuho/x:latest"))
	assert.Equal(t, `alias x='docker run --rm damianbuho/x:latest x'`, shortRef(`alias x='docker run --rm docker.io/damianbuho/x:latest x'`))
	assert.Equal(t, "docker pull ghcr.io/o/x:latest", shortRef("docker pull ghcr.io/o/x:latest"))
	assert.Equal(t, "docker pull mirror.docker.io/o/x", shortRef("docker pull mirror.docker.io/o/x"))
}
