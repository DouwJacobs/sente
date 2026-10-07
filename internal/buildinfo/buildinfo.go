// Package buildinfo exposes only public application build metadata.
package buildinfo

import (
	"runtime/debug"
	"strings"
)

// Release builds set these through -ldflags; ordinary Go builds use embedded VCS metadata.
var Version = "dev"
var Commit = ""
var BuiltAt = ""

type Info struct {
	Version  string `json:"version"`
	Commit   string `json:"commit,omitempty"`
	BuiltAt  string `json:"built_at,omitempty"`
	Modified bool   `json:"modified"`
}

func Current() Info {
	info := Info{Version: Version, Commit: Commit, BuiltAt: BuiltAt}
	if build, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range build.Settings {
			switch setting.Key {
			case "vcs.revision":
				if info.Commit == "" {
					info.Commit = setting.Value
				}
			case "vcs.time":
				if info.BuiltAt == "" {
					info.BuiltAt = setting.Value
				}
			case "vcs.modified":
				info.Modified = setting.Value == "true"
			}
		}
	}
	if strings.TrimSpace(info.Version) == "" {
		info.Version = "dev"
	}
	return info
}
