// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package releaserc

import "projectfile.org/projectfile/bridge/internal/bridge"

// No embedded template: the config is assembled directly and marshalled with
// yaml.v3 (the plugins list is mixed-shape — plain strings and [name, {config}]
// pairs — which a text/template renders only awkwardly).
func init() {
	bridge.Register(Bridge{})
}
