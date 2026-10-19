package version

import "fmt"

var (
	// Version is the semantic release version injected at build time.
	Version = "dev"
	// Commit is the git commit SHA injected at build time.
	Commit = "none"
	// Date is the build timestamp injected at build time.
	Date = "unknown"
)

// String formats the complete version string including commit and build date.
func String() string {
	return fmt.Sprintf("swapaudit %s (commit: %s, built: %s)", Version, Commit, Date)
}
