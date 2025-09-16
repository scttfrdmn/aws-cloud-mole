package version

import (
	"fmt"
	"runtime"
)

// Build-time variables set by -ldflags
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// BuildInfo contains version and build information
type BuildInfo struct {
	Version   string
	Commit    string
	Date      string
	GoVersion string
	Platform  string
}

// Get returns the current build information
func Get() BuildInfo {
	return BuildInfo{
		Version:   Version,
		Commit:    Commit,
		Date:      Date,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// String returns a formatted version string
func (bi BuildInfo) String() string {
	return fmt.Sprintf("mole version %s (%s) built on %s with %s for %s",
		bi.Version, bi.Commit[:8], bi.Date, bi.GoVersion, bi.Platform)
}