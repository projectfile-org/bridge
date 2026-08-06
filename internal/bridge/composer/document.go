// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package composer models composer.json (PHP / Packagist package manifest).
// Round-trip preservation works the same way it does for npm: the parsed
// JSON object is kept in `Rest *rawdoc.OrderedJSON` and Write paints typed
// fields back onto that canvas, so unknown keys (autoload, scripts, extra,
// repositories, ...) survive a full round-trip.
package composer

import "kiota.ch/projectfile/core/v2/pkg/rawdoc"

// Person is a composer authors[] entry. Unlike npm, composer entries are
// always object-shaped — there is no short-form string variant.
type Person struct {
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
	Homepage string `json:"homepage,omitempty"`
	Role     string `json:"role,omitempty"`
}

// Support mirrors composer's `support` sub-object. The fields we sync
// (issues, source, docs, chat, forum) flow into top-level links[] and
// repositories[]; the rest (email, wiki, irc, rss, security) round-trip
// through Rest without an explicit mapper.
type Support struct {
	Email    string `json:"email,omitempty"`
	Issues   string `json:"issues,omitempty"`
	Forum    string `json:"forum,omitempty"`
	Wiki     string `json:"wiki,omitempty"`
	IRC      string `json:"irc,omitempty"`
	Source   string `json:"source,omitempty"`
	Docs     string `json:"docs,omitempty"`
	RSS      string `json:"rss,omitempty"`
	Chat     string `json:"chat,omitempty"`
	Security string `json:"security,omitempty"`
}

// FundingEntry mirrors one element of composer's funding[]. Matches the
// projectfile.Funding shape one-to-one.
type FundingEntry struct {
	Type string `json:"type,omitempty"`
	URL  string `json:"url,omitempty"`
}

// Document is the typed view of composer.json. Heterogeneous fields
// (`license`, `prefer-stable`, `abandoned`) keep `any` because the spec
// allows multiple shapes — license is string-or-array, prefer-stable is
// bool, abandoned is bool-or-string.
type Document struct {
	Name             string            `json:"name,omitempty"`
	Description      string            `json:"description,omitempty"`
	Version          string            `json:"version,omitempty"`
	Type             string            `json:"type,omitempty"`
	Keywords         []string          `json:"keywords,omitempty"`
	Homepage         string            `json:"homepage,omitempty"`
	Readme           string            `json:"readme,omitempty"`
	Time             string            `json:"time,omitempty"`
	License          any               `json:"license,omitempty"`
	Authors          []Person          `json:"authors,omitempty"`
	Support          *Support          `json:"support,omitempty"`
	Funding          []FundingEntry    `json:"funding,omitempty"`
	Require          map[string]string `json:"require,omitempty"`
	RequireDev       map[string]string `json:"require-dev,omitempty"`
	MinimumStability string            `json:"minimum-stability,omitempty"`
	PreferStable     any               `json:"prefer-stable,omitempty"`
	Abandoned        any               `json:"abandoned,omitempty"`

	// Rest holds the raw, order-preserving view of the JSON object as
	// parsed from disk. Write paints the typed fields onto Rest so unknown
	// keys (autoload, scripts, extra, repositories, conflict, replace,
	// provide, suggest, bin, config, archive, ...) round-trip verbatim.
	Rest *rawdoc.OrderedJSON `json:"-"`
}
