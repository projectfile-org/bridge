// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package sinks materializes one package-registry link per public registry the project publishes images to.
package sinks

import (
	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/derive/containers"
	"projectfile.org/projectfile/bridge/internal/rootflags"
	"projectfile.org/projectfile/bridge/internal/scanners/core"
	"projectfile.org/projectfile/bridge/internal/scanners/forge"
	"projectfile.org/projectfile/bridge/internal/source"
)

const scannerSinks = "sinks"

func init() { core.Register(Scanner{}) }

// Scanner derives registry landing pages from org.projectfile.sinks.
type Scanner struct{}

func (Scanner) Name() string { return scannerSinks }

func (Scanner) Detect(_ string) bool { return true }

// Scan reads the merged document and proposes one concrete link per resolved public sink.
func (Scanner) Scan(root string) (*source.Partial, []core.Hit, error) {
	doc, _, err := projectfile.ReadWithOptions(root, rootflags.ReadOpts())
	if err != nil {
		return nil, nil, err
	}
	// A GHCR page hangs off the GitHub repository, so forge links staged by this same run count as declared
	staged, _ := forge.Materialize(doc)
	doc.Links = append(doc.Links, staged.Links...)
	p := &source.Partial{Links: containers.Derive(doc)}
	hits := make([]core.Hit, 0, len(p.Links))
	for _, l := range p.Links {
		hits = append(hits, core.Hit{Source: scannerSinks, Field: "links[type=" + l.Type + "]:" + l.URL})
	}
	genlog.Debug("sinks scanner: registry links derived", "links", len(p.Links))
	return p, hits, nil
}
