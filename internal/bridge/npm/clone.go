// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package npm

// Clone returns a deep-copy of the document, including its raw round-trip
// view. Used by sync/core for dry-run planning against a throwaway document.
func (d *Document) Clone() *Document {
	if d == nil {
		return nil
	}
	cp := *d
	cp.Contributors = cloneAnySlice(d.Contributors)
	cp.Maintainers = cloneAnySlice(d.Maintainers)
	cp.Keywords = cloneStrings(d.Keywords)
	cp.Dependencies = cloneStringMap(d.Dependencies)
	cp.DevDependencies = cloneStringMap(d.DevDependencies)
	cp.PeerDependencies = cloneStringMap(d.PeerDependencies)
	cp.Engines = cloneStringMap(d.Engines)
	cp.OS = cloneStrings(d.OS)
	cp.CPU = cloneStrings(d.CPU)
	cp.Author = cloneAnyValue(d.Author)
	cp.Repository = cloneAnyValue(d.Repository)
	cp.Bugs = cloneAnyValue(d.Bugs)
	cp.Funding = cloneAnyValue(d.Funding)
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

func cloneAnySlice(in []any) []any {
	if in == nil {
		return nil
	}
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = cloneAnyValue(v)
	}
	return out
}

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
	default:
		return v
	}
}
