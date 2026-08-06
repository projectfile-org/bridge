// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package cff

import "kiota.ch/projectfile/core/v2/pkg/rawdoc"

type Document struct {
	CFFVersion         string         `yaml:"cff-version"`
	Message            string         `yaml:"message"`
	Type               string         `yaml:"type"`
	Title              string         `yaml:"title"`
	Abstract           string         `yaml:"abstract"`
	Authors            []PersonEntity `yaml:"authors,omitempty"`
	Contributors       []PersonEntity `yaml:"contributors,omitempty"`
	Maintainers        []PersonEntity `yaml:"maintainers,omitempty"`
	Contact            []PersonEntity `yaml:"contact,omitempty"`
	Version            string         `yaml:"version,omitempty"`
	DateReleased       string         `yaml:"date-released,omitempty"`
	Keywords           []string       `yaml:"keywords,omitempty"`
	License            any            `yaml:"license,omitempty"`
	RepositoryCode     string         `yaml:"repository-code,omitempty"`
	URL                string         `yaml:"url,omitempty"`
	Repository         string         `yaml:"repository,omitempty"`
	RepositoryArtifact string         `yaml:"repository-artifact,omitempty"`
	DOI                string         `yaml:"doi,omitempty"`
	Identifiers        []Identifier   `yaml:"identifiers,omitempty"`
	PreferredCitation  *Reference     `yaml:"preferred-citation,omitempty"`
	LicenseURL         string         `yaml:"license-url,omitempty"`
	Commit             string         `yaml:"commit,omitempty"`
	Scope              string         `yaml:"scope,omitempty"`
	Notes              string         `yaml:"notes,omitempty"`
	References         []Reference    `yaml:"references,omitempty"`

	// Rest holds the raw, comment-preserving YAML node tree as parsed from
	// disk. Write paints the typed fields back onto it so unknown keys and
	// surrounding comments survive a round-trip.
	Rest *rawdoc.YAMLNode `yaml:"-"`

	// ReuseHeader is the REUSE/SPDX block prepended ahead of the YAML body
	// on Write. The sync driver populates it in BuildMappers (which has pf
	// available); a zero value disables the prepend so non-sync callers stay
	// header-free. Tagged yaml:"-" so it never leaks into the typed view.
	ReuseHeader string `yaml:"-"`
}

type PersonEntity struct {
	FamilyNames  string `yaml:"family-names,omitempty"`
	GivenNames   string `yaml:"given-names,omitempty"`
	NameParticle string `yaml:"name-particle,omitempty"`
	NameSuffix   string `yaml:"name-suffix,omitempty"`
	Name         string `yaml:"name,omitempty"`
	Alias        string `yaml:"alias,omitempty"`
	Email        string `yaml:"email,omitempty"`
	Affiliation  string `yaml:"affiliation,omitempty"`
	Orcid        string `yaml:"orcid,omitempty"`
	Website      string `yaml:"website,omitempty"`
	Address      string `yaml:"address,omitempty"`
	City         string `yaml:"city,omitempty"`
	Country      string `yaml:"country,omitempty"`
	Region       string `yaml:"region,omitempty"`
	PostCode     string `yaml:"post-code,omitempty"`
	Tel          string `yaml:"tel,omitempty"`
	Fax          string `yaml:"fax,omitempty"`
	Location     string `yaml:"location,omitempty"`
	DateStart    string `yaml:"date-start,omitempty"`
	DateEnd      string `yaml:"date-end,omitempty"`
}

func (p *PersonEntity) IsPerson() bool {
	return p.FamilyNames != ""
}

func (p *PersonEntity) IsEntity() bool {
	return p.FamilyNames == "" && p.Name != ""
}

type Identifier struct {
	Type  string `yaml:"type"`
	Value string `yaml:"value"`
}

type Reference struct {
	Type            string         `yaml:"type"`
	Title           string         `yaml:"title"`
	Authors         []PersonEntity `yaml:"authors,omitempty"`
	DOI             string         `yaml:"doi,omitempty"`
	Journal         string         `yaml:"journal,omitempty"`
	Volume          int            `yaml:"volume,omitempty"`
	Issue           int            `yaml:"issue,omitempty"`
	Pages           string         `yaml:"pages,omitempty"`
	Year            int            `yaml:"year,omitempty"`
	DatePublished   string         `yaml:"date-published,omitempty"`
	URL             string         `yaml:"url,omitempty"`
	Publisher       *PersonEntity  `yaml:"publisher,omitempty"`
	ISSN            string         `yaml:"issn,omitempty"`
	ISBN            string         `yaml:"isbn,omitempty"`
	Editors         []PersonEntity `yaml:"editors,omitempty"`
	CollectionDOI   string         `yaml:"collection-doi,omitempty"`
	CollectionTitle string         `yaml:"collection-title,omitempty"`
	CollectionType  string         `yaml:"collection-type,omitempty"`
	Start           int            `yaml:"start,omitempty"`
	End             int            `yaml:"end,omitempty"`
	Conference      *PersonEntity  `yaml:"conference,omitempty"`
	Institution     *PersonEntity  `yaml:"institution,omitempty"`
	License         string         `yaml:"license,omitempty"`
	Keywords        []string       `yaml:"keywords,omitempty"`
	Scope           string         `yaml:"scope,omitempty"`
	Notes           string         `yaml:"notes,omitempty"`
	Month           int            `yaml:"month,omitempty"`
}
