// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// Authority overrides live under BridgeExtensionNS as a flat `authority` map holding user overrides only; ExternalOwns reports a file-wins override for one mapped field.
func ExternalOwns(pf *projectfile.Document, bridge, filename, extKey, pfKey string) bool {
	v, ok := lookupAuthority(pf, bridge, extKey, pfKey)
	return ok && isExternalValue(v, bridge, filename)
}

// ProjectfileOwns reports an explicit opt back into pf-wins, restoring the clear-on-missing behaviour the no-destroy rule otherwise withholds.
func ProjectfileOwns(pf *projectfile.Document, bridge, extKey, pfKey string) bool {
	v, ok := lookupAuthority(pf, bridge, extKey, pfKey)
	return ok && isProjectfileValue(v)
}

// lookupAuthority tries "<bridge>.<ext-key>", "<ext-key>", "<pf-key>" in order, accepting flat keys and dotted keys walked through nested maps.
func lookupAuthority(pf *projectfile.Document, bridge, extKey, pfKey string) (string, bool) {
	if pf == nil {
		return "", false
	}
	raw, ok := projectfile.LookupExtension(pf, BridgeExtensionNS)
	if !ok {
		return "", false
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return "", false
	}
	auth, ok := m["authority"].(map[string]any)
	if !ok {
		return "", false
	}
	for _, key := range []string{bridge + "." + extKey, extKey, pfKey} {
		if key == "" || key == "." {
			continue
		}
		if v, ok := auth[key].(string); ok {
			return v, true
		}
		if v, ok := walkAuthority(auth, key); ok {
			return v, true
		}
	}
	return "", false
}

// walkAuthority resolves a dotted key through nested maps.
func walkAuthority(auth map[string]any, key string) (string, bool) {
	cur := auth
	parts := strings.Split(key, ".")
	for i, p := range parts {
		v, ok := cur[p]
		if !ok {
			return "", false
		}
		if i == len(parts)-1 {
			s, ok := v.(string)
			return s, ok
		}
		next, ok := v.(map[string]any)
		if !ok {
			return "", false
		}
		cur = next
	}
	return "", false
}

// isExternalValue maps an authority value onto file-wins; anything else (including "projectfile") does not qualify.
func isExternalValue(v, bridge, filename string) bool {
	n := strings.ToLower(strings.TrimSpace(v))
	switch n {
	case strings.ToLower(bridge), strings.ToLower(filename), "external", "ext", "file":
		return true
	}
	return false
}

// isProjectfileValue maps an authority value onto pf-wins.
func isProjectfileValue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "projectfile", "pf", "spec":
		return true
	}
	return false
}
