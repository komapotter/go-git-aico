package main

import (
	"runtime/debug"
	"strings"
)

func versionString() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	return versionFromBuildInfo(info)
}

// versionFromBuildInfo prefers the module version set by `go install ...@vX.Y.Z`
// (and therefore `@latest` on a tagged release). Local `go build` / `go run`
// typically report "(devel)"; in that case we fall back to a short VCS
// revision when Go embedded it, otherwise "devel" or "unknown".
func versionFromBuildInfo(info *debug.BuildInfo) string {
	if info == nil {
		return "unknown"
	}

	v := strings.TrimSpace(info.Main.Version)
	if v != "" && v != "(devel)" {
		return v
	}

	var revision string
	modified := false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if revision == "" {
		if v == "(devel)" {
			return "devel"
		}
		return "unknown"
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	out := "devel-" + revision
	if modified {
		out += "-dirty"
	}
	return out
}
