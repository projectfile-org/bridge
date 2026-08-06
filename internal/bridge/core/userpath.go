// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"path/filepath"

	"kiota.ch/projectfile/core/v2/pkg/userconfig"
)

// PathOrDefault resolves the on-disk write location for a renderer that
// honours a per-filename user-config override. Precedence (highest first):
//
//  1. $XDG_CONFIG_HOME/projectfile/cli.toml [generate.defaults."<filename>"].path
//  2. defaultRel — the built-in default, relative to dir
func PathOrDefault(dir, filename, defaultRel string) string {
	if rel := userconfig.PathFor(filename); rel != "" {
		return filepath.Join(dir, rel)
	}
	return filepath.Join(dir, defaultRel)
}
