// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package stack

import (
	_ "embed"
	"fmt"
	"sort"

	"go.yaml.in/yaml/v3"
)

// Rule is a single line from rules.yaml. Exactly one of File/Glob/Dir is set.
// Tags is the union contribution this rule makes; an empty slice documents a
// marker we deliberately recognise but score as zero.
type Rule struct {
	File string   `yaml:"file"`
	Glob string   `yaml:"glob"`
	Dir  string   `yaml:"dir"`
	Tags []string `yaml:"tags"`
}

//go:embed rules.yaml
var rulesData []byte

// Rules is the parsed rule table. Populated at init; never mutated afterwards.
var Rules []Rule

func init() {
	if err := yaml.Unmarshal(rulesData, &Rules); err != nil {
		// rules.yaml is embedded at compile time; a parse error here is a
		// build-the-binary problem, not a runtime one — panic is appropriate.
		panic(fmt.Sprintf("stackscan: parse embedded rules.yaml: %v", err))
	}
}

// DetectableTags returns the sorted union of every tag the embedded rule
// table can contribute. Diagnostic helper for callers that want the
// scanner's capability surface without running a scan.
func DetectableTags() []string {
	seen := map[string]struct{}{}
	for _, r := range Rules {
		for _, t := range r.Tags {
			if t == "" {
				continue
			}
			seen[t] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
