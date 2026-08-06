// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"sort"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// reservedAttributeKeys are the namespace's own keys. Every OTHER key in
// `org.projectfile.attributes` is a git attribute name carrying its pattern
// lists, so the parser has to know which two names are not attributes.
var reservedAttributeKeys = map[string]bool{"generate": true, "extra": true}

// GetAttributesExtension parses `org.projectfile.attributes`. Returns (nil, nil)
// when the namespace is absent, which the generator reads as "built-in
// vocabulary only".
func GetAttributesExtension(doc *projectfile.Document) (*AttributesExtension, error) {
	m, present, err := lookupNS(doc, AttributesExtensionNS)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	ext := &AttributesExtension{
		Generate: strListVal(m, "generate"),
		Extra:    strListVal(m, "extra"),
		Rules:    make(map[string]*AttributeRule, len(m)),
	}
	for key, v := range m {
		if reservedAttributeKeys[key] {
			continue
		}
		if rule := parseAttributeRule(v); rule != nil {
			ext.Rules[key] = rule
		}
	}
	return ext, nil
}

// AttributeNames returns the rule keys sorted. Map iteration order is
// randomised and the generated file has to be byte-stable across runs, else the
// projectfile-synced drift gate would flap on every invocation.
func (e *AttributesExtension) AttributeNames() []string {
	if e == nil {
		return nil
	}
	names := make([]string, 0, len(e.Rules))
	for name := range e.Rules {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// parseAttributeRule accepts the shorthand (a bare list means include-only) and
// the full {include, exclude} mapping, mirroring parseIgnoreTargetOverride.
func parseAttributeRule(v any) *AttributeRule {
	if list, ok := v.([]any); ok {
		return &AttributeRule{Include: toStrSlice(list)}
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	return &AttributeRule{
		Include: strListVal(m, "include"),
		Exclude: strListVal(m, "exclude"),
	}
}
