// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fundingjson

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/dlclark/regexp2"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schema/v1.1.0.json
var fundingSchemaBytes []byte

const fundingSchemaURL = "https://fundingjson.org/schema/v1.1.0.json"

var (
	schemaOnce     sync.Once
	schemaCompiled *jsonschema.Schema
	schemaErr      error
)

func loadSchema() (*jsonschema.Schema, error) {
	schemaOnce.Do(func() {
		raw, err := jsonschema.UnmarshalJSON(bytes.NewReader(fundingSchemaBytes))
		if err != nil {
			schemaErr = fmt.Errorf("parse embedded FundingJSON schema: %w", err)
			return
		}
		c := jsonschema.NewCompiler()
		c.UseRegexpEngine(ecma262Regexp)
		if err := c.AddResource(fundingSchemaURL, raw); err != nil {
			schemaErr = fmt.Errorf("register embedded FundingJSON schema: %w", err)
			return
		}
		schemaCompiled, schemaErr = c.Compile(fundingSchemaURL)
	})
	return schemaCompiled, schemaErr
}

func validateOutput(data any) error {
	s, err := loadSchema()
	if err != nil {
		return err
	}
	return s.Validate(data)
}

func ecma262Regexp(s string) (jsonschema.Regexp, error) {
	re, err := regexp2.Compile(s, regexp2.ECMAScript)
	if err != nil {
		return nil, err
	}
	return &ecmaRegexp{re: re, src: s}, nil
}

type ecmaRegexp struct {
	re  *regexp2.Regexp
	src string
}

func (r *ecmaRegexp) MatchString(s string) bool {
	m, _ := r.re.MatchString(s)
	return m
}

func (r *ecmaRegexp) String() string { return r.src }

func normalizeForValidation(doc any) (any, error) {
	b, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("normalize for validation: %w", err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, fmt.Errorf("normalize for validation: %w", err)
	}
	return v, nil
}
