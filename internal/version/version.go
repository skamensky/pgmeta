// Package version provides version information for the application.
package version

// Version is the current version of the application.
// This value is set during build time using ldflags.
var Version = "dev"

// GetVersion returns the current version of the application.
func GetVersion() string {
	return Version
}
