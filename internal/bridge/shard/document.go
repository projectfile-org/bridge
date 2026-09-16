// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package shard models shard.yml (Crystal / Shards package manifest).
package shard

import "kiota.ch/projectfile/core/v2/pkg/rawdoc"

// Document is the typed view of shard.yml for fields with a projectfile slot.
type Document struct {
	Name          string   `yaml:"name,omitempty"`
	Version       string   `yaml:"version,omitempty"`
	Description   string   `yaml:"description,omitempty"`
	Authors       []string `yaml:"authors,omitempty"`
	Crystal       string   `yaml:"crystal,omitempty"`
	License       string   `yaml:"license,omitempty"`
	Homepage      string   `yaml:"homepage,omitempty"`
	Repository    string   `yaml:"repository,omitempty"`
	Documentation string   `yaml:"documentation,omitempty"`
	// Rest holds the raw, comment-preserving YAML node tree as parsed from disk.
	Rest *rawdoc.YAMLNode `yaml:"-"`
	// ReuseHeader is the REUSE/SPDX block prepended ahead of the YAML body on Write.
	ReuseHeader string `yaml:"-"`
}
