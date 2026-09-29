// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// buildFileGroup embeds a group's declared file as its fence; an absent file drops the group.
func buildFileGroup(doc *projectfile.Document, group pfmodel.ReadmeSectionGroup, dir, section, lang string) (sectionGroupView, bool, error) {
	if group.File == "" {
		return sectionGroupView{}, false, nil
	}
	label := groupLabel(group.Name)
	body, err := readFile(dir, group.File)
	if errors.Is(err, fs.ErrNotExist) {
		genlog.DebugRow("readme_group", label, "file absent (group dropped)", group.File)
		return sectionGroupView{}, false, nil
	}
	if err != nil {
		return sectionGroupView{}, false, fmt.Errorf("bridge: read %s: %w", group.File, err)
	}
	text := stripLicenseHeader(string(body))
	if err := requireSinkRef(doc, text, group.File); err != nil {
		return sectionGroupView{}, false, err
	}
	syntax := group.Syntax
	if syntax == "" {
		syntax = syntaxDefault
	}
	genlog.DebugRow("readme_file", group.File, pfmodel.ReadmeExtensionNS+"."+section, "group="+label+" lang="+lang)
	return sectionGroupView{
		Name:     group.Name,
		Title:    expandProse(doc, sectionText(group.Title, group.TitleByLang, lang), label),
		Prefix:   expandProse(doc, sectionText(group.Prefix, group.PrefixByLang, lang), label),
		Postfix:  expandProse(doc, sectionText(group.Postfix, group.PostfixByLang, lang), label),
		Syntax:   syntax,
		Commands: strings.Split(strings.TrimRight(text, "\n"), "\n"),
	}, true, nil
}

// stripLicenseHeader drops a leading `#` comment block that carries SPDX tags, with the blank lines after it.
func stripLicenseHeader(text string) string {
	lines := strings.SplitAfter(text, "\n")
	n := 0
	for n < len(lines) && strings.HasPrefix(lines[n], "#") {
		n++
	}
	if !strings.Contains(strings.Join(lines[:n], ""), "SPDX-") {
		return text
	}
	return strings.TrimLeft(strings.Join(lines[n:], ""), "\n")
}

// requireSinkRef fails when a project publishing images embeds a file naming none of its repositories.
func requireSinkRef(doc *projectfile.Document, text, file string) error {
	sinks := readmeSinks(doc)
	if len(sinks) == 0 {
		return nil
	}
	repos := make([]string, 0, len(sinks))
	for _, s := range sinks {
		repo := sinkRepository(s.Ref)
		if strings.Contains(text, repo) {
			genlog.DebugRow("readme_file", file, "names sink "+s.Name, repo)
			return nil
		}
		repos = append(repos, repo)
	}
	return fmt.Errorf("bridge: %s names none of the published images %s", file, strings.Join(repos, ", "))
}

// sinkRepository cuts the tag off a composed reference: a pinned example stays valid across releases.
func sinkRepository(ref string) string {
	if i := strings.LastIndex(ref, ":"); i > strings.LastIndex(ref, "/") {
		return ref[:i]
	}
	return ref
}
