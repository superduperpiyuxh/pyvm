package utils

import (
	"fmt"
	"runtime/debug"
	"time"
)

// Version is overwritten at build time via ldflags:
//
//	go build -ldflags "-X github.com/user/pyvm/internal/utils.Version=1.0.0"
var Version = "dev"

// GetVersion returns the best version string available.
func GetVersion() string {
	if Version != "dev" {
		return Version
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	if bi.Main.Version != "(devel)" && bi.Main.Version != "" {
		return bi.Main.Version
	}
	var vcsRevision string
	var vcsTime time.Time
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			vcsRevision = s.Value
		case "vcs.time":
			vcsTime, _ = time.Parse(time.RFC3339, s.Value)
		case "vcs.tag":
			if s.Value != "" {
				return s.Value
			}
		}
	}
	if vcsRevision != "" {
		return fmt.Sprintf("%s (%s)", vcsRevision[:8], vcsTime.Format("2006-01-02"))
	}
	return "dev"
}
