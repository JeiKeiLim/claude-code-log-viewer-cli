// Package version provides version information for the cclv application.
package version

import "fmt"

// Version information embedded at build time via ldflags.
// Example: go build -ldflags "-X github.com/.../version.Version=v1.0.0"
var (
	// Version is the semantic version (e.g., "v1.0.0")
	Version = "dev"

	// Commit is the git commit hash
	Commit = "unknown"

	// BuildDate is the build timestamp
	BuildDate = "unknown"
)

// String returns the formatted version string.
// For release builds: "cclv v1.0.0"
// For dev builds: "cclv dev-abc1234" (first 7 chars of commit)
func String() string {
	if Version == "dev" {
		if Commit != "unknown" && len(Commit) >= 7 {
			return fmt.Sprintf("cclv dev-%s", Commit[:7])
		}
		return "cclv dev"
	}
	return fmt.Sprintf("cclv %s", Version)
}

// Full returns the full version string with all details.
func Full() string {
	return fmt.Sprintf("cclv %s (commit: %s, built: %s)", Version, Commit, BuildDate)
}
