// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Validate runs CITATION.cff 1.2.0 against its upstream JSON Schema. The
// schema is embedded so validation is offline; refresh it with
// `make sync-cff-schema`. Mirrors internal/validate (projectfile schema)
// — same compiler, same ECMA-262 regex engine.
package cff

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/dlclark/regexp2"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

//go:embed schema/v1.2.0.json
var cffSchemaBytes []byte

// cffSchemaURL is the canonical $id of the embedded CFF schema; the compiler
// keys its resource cache on this URL.
const cffSchemaURL = "https://citation-file-format.github.io/1.2.0/schema.json"

var (
	cffCompileOnce sync.Once
	cffCompiled    *jsonschema.Schema
	cffCompileErr  error
)

func loadCFFSchema() (*jsonschema.Schema, error) {
	cffCompileOnce.Do(func() {
		raw, err := jsonschema.UnmarshalJSON(bytes.NewReader(cffSchemaBytes))
		if err != nil {
			cffCompileErr = fmt.Errorf("parse embedded CFF schema: %w", err)
			return
		}
		c := jsonschema.NewCompiler()
		c.UseRegexpEngine(cffEcma262Regexp)
		if err := c.AddResource(cffSchemaURL, raw); err != nil {
			cffCompileErr = fmt.Errorf("register embedded CFF schema: %w", err)
			return
		}
		cffCompiled, cffCompileErr = c.Compile(cffSchemaURL)
	})
	return cffCompiled, cffCompileErr
}

// Validate parses raw as YAML/JSON and runs it against the CFF 1.2.0
// schema. Returns a wrapped error whose ValidationError (when present)
// carries per-field paths the caller can pretty-print.
func Validate(raw []byte) error {
	s, err := loadCFFSchema()
	if err != nil {
		return err
	}
	// CFF on disk is YAML, but santhosh's validator wants the JSON data
	// model. The cff.Document struct already round-trips through gopkg.in/
	// yaml.v3; the cheapest path to the JSON model is parse-to-Document
	// then encode-to-JSON. That keeps the validator agnostic to YAML
	// quirks like alias collapse or non-string map keys.
	doc, err := parseForValidation(raw)
	if err != nil {
		return fmt.Errorf("parse CFF for validation: %w", err)
	}
	return s.Validate(doc)
}

// parseForValidation decodes raw via yaml then re-marshals through
// encoding/json to land on (string-keyed) generic types the schema
// validator understands. yaml.v3 may produce map[string]interface{}
// directly for plain mappings, but it does not for nested non-string
// keys; the json round-trip normalises both cases.
func parseForValidation(raw []byte) (any, error) {
	var via any
	if err := yaml.Unmarshal(raw, &via); err != nil {
		return nil, err
	}
	b, err := json.Marshal(via)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func cffEcma262Regexp(s string) (jsonschema.Regexp, error) {
	re, err := regexp2.Compile(s, regexp2.ECMAScript)
	if err != nil {
		return nil, err
	}
	return &cffEcmaRegexp{re: re, src: s}, nil
}

type cffEcmaRegexp struct {
	re  *regexp2.Regexp
	src string
}

func (r *cffEcmaRegexp) MatchString(s string) bool {
	m, _ := r.re.MatchString(s)
	return m
}

func (r *cffEcmaRegexp) String() string { return r.src }
