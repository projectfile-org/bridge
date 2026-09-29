// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// artifactKindAction is the kind of a GitHub Action artifact, whose `path` names its action.yaml.
const artifactKindAction = "action"

// actionInput is one row of the usage block's inputs table.
type actionInput struct {
	Name        string
	Default     string
	Required    bool
	Description string
}

// actionInputs reads the inputs of every action artifact's action.yaml, in file order.
func actionInputs(doc *projectfile.Document, dir string) []actionInput {
	artifacts, _ := pfmodel.GetArtifacts(doc)
	var out []actionInput
	for _, a := range pfmodel.ArtifactsOfKind(artifacts, artifactKindAction) {
		if a.Path == "" {
			continue
		}
		body, err := readFile(dir, a.Path)
		if err != nil {
			genlog.DebugRow("action_inputs", a.Path, "unreadable (skipped)", err.Error())
			continue
		}
		var root struct {
			Inputs yaml.Node `yaml:"inputs"`
		}
		if err := yaml.Unmarshal(body, &root); err != nil || root.Inputs.Kind != yaml.MappingNode {
			genlog.DebugRow("action_inputs", a.Path, "no inputs mapping (skipped)", a.Key)
			continue
		}
		for i := 0; i+1 < len(root.Inputs.Content); i += 2 {
			var in struct {
				Description string `yaml:"description"`
				Default     string `yaml:"default"`
				Required    bool   `yaml:"required"`
			}
			if err := root.Inputs.Content[i+1].Decode(&in); err != nil {
				genlog.DebugRow("action_input", root.Inputs.Content[i].Value, "undecodable (skipped)", a.Path)
				continue
			}
			out = append(out, actionInput{
				Name:        root.Inputs.Content[i].Value,
				Default:     in.Default,
				Required:    in.Required,
				Description: strings.ReplaceAll(strings.Join(strings.Fields(in.Description), " "), "|", `\|`),
			})
		}
		genlog.DebugRow("action_inputs", a.Path, "inputs read", a.Key+" total="+strconv.Itoa(len(out)))
	}
	return out
}
