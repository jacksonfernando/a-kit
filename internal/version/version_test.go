package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGet_DefaultsToDevBuild(t *testing.T) {
	// When no ldflags are injected and there's no build info version,
	// Get() should return "dev".
	original := Version
	Version = "dev"
	defer func() { Version = original }()

	result := Get()
	// In test context (go test) debug.ReadBuildInfo returns "(devel)" or empty
	// so we always expect "dev" here.
	assert.Equal(t, "dev", result)
}

func TestGet_ReturnsInjectedVersion(t *testing.T) {
	original := Version
	Version = "v1.2.3"
	defer func() { Version = original }()

	assert.Equal(t, "v1.2.3", Get())
}

func TestGet_SemverFormat(t *testing.T) {
	cases := []struct {
		version string
		want    string
	}{
		{"v1.0.0", "v1.0.0"},
		{"v2.3.14", "v2.3.14"},
		{"v0.1.0-beta", "v0.1.0-beta"},
		{"dev", "dev"},
	}

	original := Version
	defer func() { Version = original }()

	for _, tc := range cases {
		t.Run(tc.version, func(t *testing.T) {
			Version = tc.version
			assert.Equal(t, tc.want, Get())
		})
	}
}
