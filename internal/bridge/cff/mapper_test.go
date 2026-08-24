// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package cff

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

const (
	testFamilyName  = "Búho"
	testGivenName   = "Damián"
	testAuthorEmail = "damian@example.org"
	testAffiliation = "Independent"
	testHomepage    = "https://dbuho.me"
	testAuthorRole  = "author"
	testOrgName     = "TODO Group"
	testOrgURL      = "https://todogroup.org/"
	testTitle       = "Test"
	testMIT         = "MIT"
	testMITOrApache = "MIT OR Apache-2.0"
	testApache      = "Apache-2.0"
)

// TestPFPeopleToCFF_ShapeDiscrimination covers the producer-side rule:
// person-only fields never appear on an entity record, entity-only `name`
// never appears on a person. Each case is checked twice — once on the
// shape of the produced PersonEntity, once by passing the synthesised
// CITATION.cff body through Validate, which is the authoritative gate.
func TestPFPeopleToCFF_ShapeDiscrimination(t *testing.T) {
	cases := []struct {
		name   string
		in     []projectfile.Person
		assert func(t *testing.T, out []PersonEntity)
	}{
		{
			name: "pure_person",
			in: []projectfile.Person{{
				FamilyNames: testFamilyName,
				GivenNames:  testGivenName,
				Email:       testAuthorEmail,
				Affiliation: testAffiliation,
				Orcid:       "0009-0004-8579-7633",
				URL:         testHomepage,
				Roles:       []string{testAuthorRole},
			}},
			assert: func(t *testing.T, out []PersonEntity) {
				if len(out) != 1 {
					t.Fatalf("want 1 record, got %d", len(out))
				}
				p := out[0]
				if p.Name != "" {
					t.Errorf("person carried entity-only Name=%q", p.Name)
				}
				if p.FamilyNames != testFamilyName || p.GivenNames != testGivenName {
					t.Errorf("person name fields lost: %+v", p)
				}
				if p.Affiliation != testAffiliation {
					t.Errorf("person affiliation lost: %+v", p)
				}
				if p.Orcid != "https://orcid.org/0009-0004-8579-7633" {
					t.Errorf("orcid not normalised: %q", p.Orcid)
				}
				if p.Website != testHomepage {
					t.Errorf("url→website lost: %q", p.Website)
				}
			},
		},
		{
			name: "unidentified_dropped",
			in: []projectfile.Person{
				{Roles: []string{testAuthorRole}},
				{FamilyNames: "Solo"},
			},
			assert: func(t *testing.T, out []PersonEntity) {
				if len(out) != 1 || out[0].FamilyNames != "Solo" {
					t.Errorf("want only the named entry, got %+v", out)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := pfPeopleToCFF(tc.in)
			tc.assert(t, out)
			assertSynthesisedCFFValidates(t, out)
		})
	}
}

func TestPFOrgsToCFF_ShapeDiscrimination(t *testing.T) {
	cases := []struct {
		name   string
		in     []projectfile.Organization
		assert func(t *testing.T, out []PersonEntity)
	}{
		{
			name: "pure_entity",
			in: []projectfile.Organization{{
				Name:  testOrgName,
				URL:   testOrgURL,
				Roles: []string{"founder"},
			}},
			assert: func(t *testing.T, out []PersonEntity) {
				if len(out) != 1 {
					t.Fatalf("want 1 record, got %d", len(out))
				}
				e := out[0]
				if e.FamilyNames != "" || e.GivenNames != "" || e.NameParticle != "" || e.NameSuffix != "" || e.Affiliation != "" {
					t.Errorf("entity carried person-only fields: %+v", e)
				}
				if e.Name != testOrgName {
					t.Errorf("entity Name lost: %q", e.Name)
				}
				if e.Website != testOrgURL {
					t.Errorf("entity website lost: %q", e.Website)
				}
			},
		},
		{
			name: "unidentified_dropped",
			in: []projectfile.Organization{
				{Roles: []string{testAuthorRole}},
				{Name: "Solo Org"},
			},
			assert: func(t *testing.T, out []PersonEntity) {
				if len(out) != 1 || out[0].Name != "Solo Org" {
					t.Errorf("want only the named entry, got %+v", out)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := pfOrgsToCFF(tc.in)
			tc.assert(t, out)
			assertSynthesisedCFFValidates(t, out)
		})
	}
}

// TestCFFPeopleToPF_ShapeDiscrimination is the inverse — entity input
// never produces a PF person record with person-only fields and vice
// versa. Mixed-shape input (invalid per upstream but possible from
// hand-edited CITATION.cff) collapses to person, since FamilyNames is
// the more specific discriminator.
func TestCFFPeopleToPF_ShapeDiscrimination(t *testing.T) {
	in := []PersonEntity{
		{FamilyNames: testFamilyName, GivenNames: testGivenName, Affiliation: testAffiliation, Email: testAuthorEmail},
		{Name: testOrgName, Website: testOrgURL},
		{FamilyNames: "Stray", Name: "Wrongly-Set"},
		{Email: "no-name@x"},
	}
	people, orgs := cffPeopleToPF(in, testAuthorRole)
	if len(people) != 2 {
		t.Fatalf("want 2 people (unidentified dropped, mixed→person), got %d: %+v", len(people), people)
	}
	if len(orgs) != 1 {
		t.Fatalf("want 1 org, got %d: %+v", len(orgs), orgs)
	}
	if people[0].Affiliation != testAffiliation {
		t.Errorf("person affiliation lost: %+v", people[0])
	}
	if orgs[0].URL != testOrgURL {
		t.Errorf("entity website→url lost: %+v", orgs[0])
	}
	if people[1].FamilyNames != "Stray" {
		t.Errorf("mixed should collapse to person, got %+v", people[1])
	}
}

// TestRoundTrip_PFToCFFToPF guarantees a person stays a person and an
// entity stays an entity across the bridge. Without the shape-aware
// copy, the entity would round-trip with a phantom Affiliation field
// or the person would emerge with both Name and FamilyNames.
func TestRoundTrip_PFToCFFToPF(t *testing.T) {
	peopleIn := []projectfile.Person{
		{FamilyNames: testFamilyName, GivenNames: testGivenName, Email: testAuthorEmail, Orcid: "0009-0004-8579-7633", URL: testHomepage, Affiliation: testAffiliation, Roles: []string{testAuthorRole}},
	}
	orgsIn := []projectfile.Organization{
		{Name: testOrgName, URL: testOrgURL, Roles: []string{testAuthorRole}},
	}
	cffSide := append(pfPeopleToCFF(peopleIn), pfOrgsToCFF(orgsIn)...)
	peopleBack, orgsBack := cffPeopleToPF(cffSide, testAuthorRole)
	if len(peopleBack) != 1 {
		t.Fatalf("want 1 person back, got %d", len(peopleBack))
	}
	if peopleBack[0].FamilyNames != testFamilyName || peopleBack[0].Affiliation != testAffiliation {
		t.Errorf("person round-trip drifted: %+v", peopleBack[0])
	}
	if len(orgsBack) != 1 {
		t.Fatalf("want 1 org back, got %d", len(orgsBack))
	}
	if orgsBack[0].Name != testOrgName {
		t.Errorf("entity round-trip drifted: %+v", orgsBack[0])
	}
	assertSynthesisedCFFValidates(t, cffSide)
}

// TestValidate_RejectsMixedShape pins the runtime gate — the validator
// must reject a hand-crafted authors list with both name and family-names
// on one record. If this ever passes, the bug class is back.
func TestValidate_RejectsMixedShape(t *testing.T) {
	body := strings.Join([]string{
		"cff-version: 1.2.0",
		"message: test",
		"title: Test",
		"type: software",
		"authors:",
		"  - family-names: Búho",
		"    name: Wrongly-Set",
		"",
	}, "\n")
	err := Validate([]byte(body))
	if err == nil {
		t.Fatal("expected validation failure for mixed-shape author, got nil")
	}
}

// assertSynthesisedCFFValidates builds a minimal CITATION.cff body around
// the given authors slice and runs it through the embedded schema. Useful
// when the test focus is the mapper's shape, but the validator is the
// authoritative source of truth for "does this conform to CFF 1.2.0".
func assertSynthesisedCFFValidates(t *testing.T, authors []PersonEntity) {
	t.Helper()
	if len(authors) == 0 {
		return
	}
	doc := &Document{
		CFFVersion: "1.2.0",
		Message:    "test",
		Type:       "software",
		Title:      testTitle,
		Abstract:   "Test abstract.",
		Authors:    authors,
	}
	data, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal synthesised doc: %v", err)
	}
	if err := Validate(data); err != nil {
		t.Fatalf("synthesised CFF rejected by validator: %v\n---\n%s", err, data)
	}
}

// TestLicense_FromPF_OR_EmitsArray verifies that an OR compound SPDX
// expression produces a CFF license array (CFF 1.2.0 multi-license =
// OR semantics), and that the result validates against the schema.
func TestLicense_FromPF_OR_EmitsArray(t *testing.T) {
	c := New()
	pf := &projectfile.Document{
		License: &projectfile.License{Spdx: testMITOrApache},
	}
	mappers := buildMappers(c, pf)
	licenseMapper := findMapper(t, mappers, "license")

	report := licenseMapper.FromPF(true)
	if report == "" {
		t.Fatal("FromPF returned empty for OR compound — expected an array emission")
	}
	arr, ok := c.License.([]string)
	if !ok {
		t.Fatalf("expected []string for OR compound, got %T: %v", c.License, c.License)
	}
	if len(arr) != 2 || arr[0] != testMIT || arr[1] != testApache {
		t.Fatalf("expected [MIT Apache-2.0], got %v", arr)
	}

	// Verify the emitted document validates.
	c.Title = testTitle
	c.Abstract = "Test abstract."
	c.Authors = []PersonEntity{{FamilyNames: testTitle}}
	data, err := yaml.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := Validate(data); err != nil {
		t.Fatalf("OR-array CFF rejected by validator: %v\n---\n%s", err, data)
	}
}

// TestLicense_FromPF_AND_Skips verifies that AND compound expressions
// are skipped (CFF cannot represent conjunctive licenses).
func TestLicense_FromPF_AND_Skips(t *testing.T) {
	c := New()
	pf := &projectfile.Document{
		License: &projectfile.License{Spdx: "MIT AND Apache-2.0"},
	}
	mappers := buildMappers(c, pf)
	licenseMapper := findMapper(t, mappers, "license")

	report := licenseMapper.FromPF(true)
	if report != "" {
		t.Fatalf("expected empty report for AND compound, got %q", report)
	}
	if c.License != nil {
		t.Fatalf("expected nil License for AND compound, got %v", c.License)
	}
}

// TestLicense_FromPF_Single_String verifies the existing single-license
// path still emits a scalar string (unchanged behaviour).
func TestLicense_FromPF_Single_String(t *testing.T) {
	c := New()
	pf := &projectfile.Document{
		License: &projectfile.License{Spdx: testMIT},
	}
	mappers := buildMappers(c, pf)
	licenseMapper := findMapper(t, mappers, "license")

	report := licenseMapper.FromPF(true)
	if report != testMIT {
		t.Fatalf("expected 'MIT', got %q", report)
	}
	s, ok := c.License.(string)
	if !ok || s != testMIT {
		t.Fatalf("expected string 'MIT', got %T: %v", c.License, c.License)
	}
}

// TestLicense_ToPF_ArrayToSPDX verifies that a CFF license array reads
// back into projectfile as an "X OR Y" SPDX expression.
func TestLicense_ToPF_ArrayToSPDX(t *testing.T) {
	c := New()
	c.License = []string{testMIT, testApache}
	pf := &projectfile.Document{}

	mappers := buildMappers(c, pf)
	licenseMapper := findMapper(t, mappers, "license")

	report := licenseMapper.ToPF(true)
	if report != testMITOrApache {
		t.Fatalf("expected 'MIT OR Apache-2.0', got %q", report)
	}
	if pf.License == nil || pf.License.Spdx != testMITOrApache {
		t.Fatalf("pf.License.Spdx = %q, want 'MIT OR Apache-2.0'", spdxOrZero(pf))
	}
}

// TestLicenseToString covers the normalisation helper for all input shapes.
func TestLicenseToString(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"nil", nil, ""},
		{"string", testMIT, testMIT},
		{"[]string", []string{testMIT, testApache}, testMITOrApache},
		{"[]any", []any{testMIT, testApache}, testMITOrApache},
		{"empty_string", "", ""},
		{"empty_slice", []string{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := LicenseToString(tc.in)
			if got != tc.want {
				t.Errorf("LicenseToString(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func findMapper(t *testing.T, ml core.MapperList, extKey string) core.FieldMapper {
	t.Helper()
	for _, m := range ml {
		if m.ExtKey == extKey {
			return m
		}
	}
	t.Fatalf("mapper %q not found", extKey)
	return core.FieldMapper{}
}

func spdxOrZero(pf *projectfile.Document) string {
	if pf.License == nil {
		return ""
	}
	return pf.License.Spdx
}
