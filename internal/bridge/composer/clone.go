// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package composer

// Clone returns a deep-copy of the document, including its raw round-trip
// view. Used by sync/core for dry-run planning against a throwaway document.
func (d *Document) Clone() *Document {
	if d == nil {
		return nil
	}
	cp := *d
	cp.Keywords = cloneStrings(d.Keywords)
	cp.Authors = clonePeople(d.Authors)
	cp.Funding = cloneFunding(d.Funding)
	cp.Require = cloneStringMap(d.Require)
	cp.RequireDev = cloneStringMap(d.RequireDev)
	cp.License = cloneAnyValue(d.License)
	cp.PreferStable = cloneAnyValue(d.PreferStable)
	cp.Abandoned = cloneAnyValue(d.Abandoned)
	if d.Support != nil {
		s := *d.Support
		cp.Support = &s
	}
	cp.Rest = d.Rest.Clone()
	return &cp
}

func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func clonePeople(in []Person) []Person {
	if in == nil {
		return nil
	}
	out := make([]Person, len(in))
	copy(out, in)
	return out
}

func cloneFunding(in []FundingEntry) []FundingEntry {
	if in == nil {
		return nil
	}
	out := make([]FundingEntry, len(in))
	copy(out, in)
	return out
}

// cloneAnyValue mirrors npm's helper — license / prefer-stable / abandoned
// can carry maps or arrays of any, so deep-copy nested containers if present.
func cloneAnyValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[k] = cloneAnyValue(vv)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, vv := range x {
			out[i] = cloneAnyValue(vv)
		}
		return out
	case []string:
		out := make([]string, len(x))
		copy(out, x)
		return out
	default:
		return v
	}
}
