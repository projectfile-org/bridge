// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package releaserc

// releaseConfig is the marshalled shape of a semantic-release `.releaserc.yaml`.
// Field order is the emit order (kept stable for a clean, reviewable diff):
// tagFormat, branches, plugins.
type releaseConfig struct {
	TagFormat string        `yaml:"tagFormat"`
	Branches  []branchEntry `yaml:"branches"`
	Plugins   []any         `yaml:"plugins"`
}

// branchEntry is one semantic-release branch. channel/prerelease are omitted
// when unset so a plain release branch renders as just `- name: main`.
type branchEntry struct {
	Name       string `yaml:"name"`
	Channel    string `yaml:"channel,omitempty"`
	Prerelease bool   `yaml:"prerelease,omitempty"`
}
