package version

import "runtime/debug"

// Version is injected at build time via:
//
//	go build -ldflags "-X github.com/jacksonfernando/a-kit/internal/version.Version=v1.0.0"
//
// When installed via `go install`, it falls back to the module version from build info.
var Version = "dev"

// Get returns the effective version string.
func Get() string {
	if Version != "dev" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok &&
		info.Main.Version != "" &&
		info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
