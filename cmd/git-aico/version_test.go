package main

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestVersionFromBuildInfo(t *testing.T) {
	tests := []struct {
		name string
		info *debug.BuildInfo
		want string
	}{
		{
			name: "nil info",
			info: nil,
			want: "unknown",
		},
		{
			name: "tagged module version from go install",
			info: &debug.BuildInfo{Main: debug.Module{Version: "v0.3.2"}},
			want: "v0.3.2",
		},
		{
			name: "pseudo-version is kept",
			info: &debug.BuildInfo{Main: debug.Module{Version: "v0.3.2-0.20260920123456-abcdefabcdef"}},
			want: "v0.3.2-0.20260920123456-abcdefabcdef",
		},
		{
			name: "devel without vcs",
			info: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}},
			want: "devel",
		},
		{
			name: "devel with revision",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "(devel)"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "7c84fa3abcdeffff"},
				},
			},
			want: "devel-7c84fa3",
		},
		{
			name: "devel with dirty revision",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "(devel)"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "7c84fa3abcdeffff"},
					{Key: "vcs.modified", Value: "true"},
				},
			},
			want: "devel-7c84fa3-dirty",
		},
		{
			name: "empty version without vcs",
			info: &debug.BuildInfo{},
			want: "unknown",
		},
		{
			name: "empty version with vcs",
			info: &debug.BuildInfo{
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abc1234deadbeef"},
				},
			},
			want: "devel-abc1234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := versionFromBuildInfo(tt.info)
			if got != tt.want {
				t.Fatalf("versionFromBuildInfo() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVersionStringNotEmpty(t *testing.T) {
	got := versionString()
	if strings.TrimSpace(got) == "" {
		t.Fatal("versionString() returned an empty string")
	}
}
