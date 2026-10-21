// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// TestArtifactsAreSortedByNameAndCarryTheirKey pins the name order the resolver and the README share, and the key riding on the value.
func TestArtifactsAreSortedByNameAndCarryTheirKey(t *testing.T) {
	ext := map[string]any{
		artifactKeyCLI: map[string]any{
			keyKind: ArtifactKindBinary, "summary": "the reader",
			"path": "dist/cli", "command": "pf-cli", "version": refV1,
			"architecture": []any{archAMD64}, "operating-system": []any{osLinux},
		},
		"pkg": map[string]any{
			keyKind: ArtifactKindPackage, keyName: "@scope/thing", "registry": targetNpm,
			"summary": map[string]any{"en": "npm package", "es": "paquete npm"},
		},
		artifactKeyWeb: map[string]any{keyKind: ArtifactKindWebsite, fxURL: urlXTest},
		"broken":       "not-a-map",
	}
	arts, err := GetArtifacts(extDoc(ArtifactsExtensionNS, ext))
	require.NoError(t, err)
	require.Len(t, arts, 3, "a malformed entry is skipped rather than fatal")
	assert.Equal(t, []string{artifactKeyCLI, "pkg", artifactKeyWeb}, []string{arts[0].Key, arts[1].Key, arts[2].Key})
	assert.Equal(t, "the reader", arts[0].Summary)
	assert.Equal(t, "dist/cli", arts[0].Path)
	assert.Equal(t, "pf-cli", arts[0].Command)
	assert.Equal(t, refV1, arts[0].Version)
	assert.Equal(t, []string{archAMD64}, arts[0].Architecture)
	assert.Equal(t, []string{osLinux}, arts[0].OperatingSystem)
	assert.Equal(t, "npm package", arts[1].Summary)
	assert.Equal(t, map[string]string{"en": "npm package", "es": "paquete npm"}, arts[1].SummaryByLang)
	assert.Equal(t, "@scope/thing", arts[1].Name, "a package's published name is not the map key")
	assert.Equal(t, targetNpm, arts[1].Registry)
}

// TestGetArtifactsAbsentIsNothing: a project that declares nothing produces nothing a generator can describe, which is a valid state, not an error.
func TestGetArtifactsAbsentIsNothing(t *testing.T) {
	arts, err := GetArtifacts(&projectfile.Document{})
	require.NoError(t, err)
	assert.Empty(t, arts)
}

// TestArtifactsOfKindFiltersAndPreservesOrder is the Go-side twin of the `{kind=…}` address, so the inventory block and a command recipe select the same set.
func TestArtifactsOfKindFiltersAndPreservesOrder(t *testing.T) {
	arts := []Artifact{
		{Key: "a", Kind: ArtifactKindBinary},
		{Key: "b", Kind: ArtifactKindImage},
		{Key: "c", Kind: ArtifactKindBinary},
	}
	got := ArtifactsOfKind(arts, ArtifactKindBinary)
	require.Len(t, got, 2)
	assert.Equal(t, "a", got[0].Key)
	assert.Equal(t, "c", got[1].Key)
	assert.Empty(t, ArtifactsOfKind(arts, ArtifactKindService))
}

// TestParseArtifactPortsAcceptsBareNumbersAndMaps pins the one-line form and the named form, and the refusal of a port that is not a number.
func TestParseArtifactPortsAcceptsBareNumbersAndMaps(t *testing.T) {
	ports := parseArtifactPorts(map[string]any{"ports": []any{
		3306,
		float64(8080),
		int64(9090),
		map[string]any{"port": 6379, keyName: keyMetrics, "protocol": "tcp"},
		map[string]any{keyName: "no port"},
		junkRow,
	}})
	require.Len(t, ports, 4)
	assert.Equal(t, ArtifactPort{Port: 3306}, ports[0])
	assert.Equal(t, ArtifactPort{Port: 8080}, ports[1], "JSON decoders hand back float64")
	assert.Equal(t, ArtifactPort{Port: 9090}, ports[2], "TOML decoders hand back int64")
	assert.Equal(t, ArtifactPort{Port: 6379, Name: keyMetrics, Protocol: "tcp"}, ports[3])
	assert.Nil(t, parseArtifactPorts(map[string]any{"ports": notAMapping}))
	assert.Nil(t, parseArtifactPorts(map[string]any{}))
}

// TestAsIntCoercesAndRefuses pins the coercion and the refusal: port 0 is not a port, so a non-numeric value is refused rather than defaulted.
func TestAsIntCoercesAndRefuses(t *testing.T) {
	n, ok := asInt(7)
	assert.True(t, ok)
	assert.Equal(t, 7, n)
	n, ok = asInt(int64(8))
	assert.True(t, ok)
	assert.Equal(t, 8, n)
	n, ok = asInt(float64(9.5))
	assert.False(t, ok, "9.5 is not a port number")
	assert.Equal(t, 9, n, "the truncated value rides along, so a caller that ignores ok sees 9")
	n, ok = asInt(float64(9))
	assert.True(t, ok)
	assert.Equal(t, 9, n)
	_, ok = asInt("9")
	assert.False(t, ok, "a numeric string is refused")
	_, ok = asInt(nil)
	assert.False(t, ok)
}
