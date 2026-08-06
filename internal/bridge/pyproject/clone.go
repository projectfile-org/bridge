// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pyproject

// Clone returns a deep-copy of the document, including its raw round-trip
// view. Used by sync/core for dry-run planning against a throwaway document.
func (d *Document) Clone() *Document {
	if d == nil {
		return nil
	}
	cp := *d
	cp.Project = cloneProject(d.Project)
	cp.Rest = d.Rest.Clone()
	return &cp
}

func cloneProject(p Project) Project {
	cp := p
	cp.ReadMe = cloneAnyValue(p.ReadMe)
	cp.License = cloneAnyValue(p.License)
	cp.LicenseFiles = cloneStrings(p.LicenseFiles)
	cp.Authors = clonePersons(p.Authors)
	cp.Maintainers = clonePersons(p.Maintainers)
	cp.Keywords = cloneStrings(p.Keywords)
	cp.Classifiers = cloneStrings(p.Classifiers)
	cp.URLs = cloneStringMap(p.URLs)
	cp.Scripts = cloneStringMap(p.Scripts)
	cp.GUIScripts = cloneStringMap(p.GUIScripts)
	cp.EntryPoints = cloneEntryPoints(p.EntryPoints)
	cp.Dependencies = cloneStrings(p.Dependencies)
	cp.OptionalDependencies = cloneOptionalDeps(p.OptionalDependencies)
	cp.Dynamic = cloneStrings(p.Dynamic)
	return cp
}

func clonePersons(in []Person) []Person {
	if in == nil {
		return nil
	}
	out := make([]Person, len(in))
	copy(out, in)
	return out
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

func cloneEntryPoints(in map[string]map[string]string) map[string]map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]map[string]string, len(in))
	for group, m := range in {
		out[group] = cloneStringMap(m)
	}
	return out
}

func cloneOptionalDeps(in map[string][]string) map[string][]string {
	if in == nil {
		return nil
	}
	out := make(map[string][]string, len(in))
	for group, deps := range in {
		out[group] = cloneStrings(deps)
	}
	return out
}

// cloneAnyValue mirrors npm's helper — `readme` and `license` can be string,
// {file: ...}, or {text: ..., content-type: ...}, so we have to deep-copy
// nested maps if present.
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
