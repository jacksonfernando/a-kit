package version

import "runtime/debug"

const (
	devVersion   = "dev"
	develVersion = "(devel)"
)

// Version is injected at build time via:
//
//	go build -ldflags "-X github.com/jacksonfernando/a-kit/internal/version.Version=v1.0.0"
//
// When installed via `go install`, it falls back to the module version from build info.
var Version = devVersion

// Get returns the effective version string.
func Get() string {
	if Version != devVersion {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok &&
		info.Main.Version != "" &&
		info.Main.Version != develVersion {
		return info.Main.Version
	}
	return devVersion
}
