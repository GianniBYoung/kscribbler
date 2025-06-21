package version

// these are set by -ldflags on release
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)
