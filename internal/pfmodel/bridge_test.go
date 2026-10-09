// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

const (
	testBridgeName     = "pyproject"
	testBridgeFilename = "pyproject.toml"
	testExtKey         = "requires-python"
	testPFKey          = "requirements.runtime.python"
)

func setAuthority(pf *projectfile.Document, auth map[string]any) {
	projectfile.SetExtension(pf, BridgeExtensionNS, map[string]any{"authority": auth})
}

func TestAuthorityAbsentMeansNoOwner(t *testing.T) {
	pf := &projectfile.Document{}
	assert.False(t, ExternalOwns(pf, testBridgeName, testBridgeFilename, testExtKey, testPFKey))
	assert.False(t, ProjectfileOwns(pf, testBridgeName, testExtKey, testPFKey))
}

func TestAuthorityBridgeNameMeansExternal(t *testing.T) {
	pf := &projectfile.Document{}
	setAuthority(pf, map[string]any{testExtKey: testBridgeName})
	assert.True(t, ExternalOwns(pf, testBridgeName, testBridgeFilename, testExtKey, testPFKey))
	assert.False(t, ProjectfileOwns(pf, testBridgeName, testExtKey, testPFKey))
}

func TestAuthorityAcceptsFilenameAndGenericAliases(t *testing.T) {
	for _, v := range []string{testBridgeFilename, "external", "ext", "file"} {
		pf := &projectfile.Document{}
		setAuthority(pf, map[string]any{testExtKey: v})
		assert.True(t, ExternalOwns(pf, testBridgeName, testBridgeFilename, testExtKey, testPFKey), "value %q", v)
	}
}

func TestAuthorityProjectfileRestoresPFWins(t *testing.T) {
	for _, v := range []string{"projectfile", "pf", "spec"} {
		pf := &projectfile.Document{}
		setAuthority(pf, map[string]any{testExtKey: v})
		assert.True(t, ProjectfileOwns(pf, testBridgeName, testExtKey, testPFKey), "value %q", v)
		assert.False(t, ExternalOwns(pf, testBridgeName, testBridgeFilename, testExtKey, testPFKey), "value %q", v)
	}
}

func TestAuthorityKeysByPFKeyAndBridgePrefix(t *testing.T) {
	pf := &projectfile.Document{}
	setAuthority(pf, map[string]any{testPFKey: testBridgeName})
	assert.True(t, ExternalOwns(pf, testBridgeName, testBridgeFilename, testExtKey, testPFKey))

	prefixed := &projectfile.Document{}
	setAuthority(prefixed, map[string]any{"pyproject.requires-python": "projectfile"})
	assert.True(t, ProjectfileOwns(prefixed, testBridgeName, testExtKey, testPFKey))
}

func TestAuthorityUnknownValueOwnsNothing(t *testing.T) {
	pf := &projectfile.Document{}
	setAuthority(pf, map[string]any{testExtKey: "someone-else"})
	assert.False(t, ExternalOwns(pf, testBridgeName, testBridgeFilename, testExtKey, testPFKey))
	assert.False(t, ProjectfileOwns(pf, testBridgeName, testExtKey, testPFKey))
}
