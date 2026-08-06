// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package npm

import "kiota.ch/projectfile/core/v2/pkg/rawdoc"

type Person struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

type Repository struct {
	URL       string `json:"url,omitempty"`
	Type      string `json:"type,omitempty"`
	Directory string `json:"directory,omitempty"`
}

type Bugs struct {
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

type FundingEntry struct {
	Type string `json:"type,omitempty"`
	URL  string `json:"url,omitempty"`
}

type Document struct {
	Name             string            `json:"name,omitempty"`
	Version          string            `json:"version,omitempty"`
	Description      string            `json:"description,omitempty"`
	License          string            `json:"license,omitempty"`
	Author           any               `json:"author,omitempty"`
	Contributors     []any             `json:"contributors,omitempty"`
	Maintainers      []any             `json:"maintainers,omitempty"`
	Keywords         []string          `json:"keywords,omitempty"`
	Homepage         string            `json:"homepage,omitempty"`
	Repository       any               `json:"repository,omitempty"`
	Bugs             any               `json:"bugs,omitempty"`
	Dependencies     map[string]string `json:"dependencies,omitempty"`
	DevDependencies  map[string]string `json:"devDependencies,omitempty"`
	PeerDependencies map[string]string `json:"peerDependencies,omitempty"`
	Engines          map[string]string `json:"engines,omitempty"`
	OS               []string          `json:"os,omitempty"`
	CPU              []string          `json:"cpu,omitempty"`
	Funding          any               `json:"funding,omitempty"`
	Private          bool              `json:"private,omitempty"`

	// Rest holds the raw, order-preserving view of the JSON object as parsed
	// from disk. It is the canvas onto which Write paints the typed fields,
	// so unknown keys (scripts, type, exports, volta, workspaces, pnpm, etc.)
	// survive a full round-trip — even for documents created from scratch.
	Rest *rawdoc.OrderedJSON `json:"-"`
}
