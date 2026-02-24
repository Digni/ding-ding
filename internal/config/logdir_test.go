package config

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestResolveDefaultLogDir(t *testing.T) {
	tests := []struct {
		name     string
		opts     logDirResolverOptions
		wantPath string
	}{
		{
			name: "darwin uses library logs path",
			opts: logDirResolverOptions{
				GOOS: "darwin",
				userHomeDir: func() (string, error) {
					return "/Users/alex", nil
				},
			},
			wantPath: filepath.Join("/Users/alex", "Library", "Logs", "ding-ding"),
		},
		{
			name: "linux uses xdg state home when set",
			opts: logDirResolverOptions{
				GOOS: "linux",
				getenv: func(key string) string {
					if key == "XDG_STATE_HOME" {
						return "/state"
					}
					return ""
				},
				userHomeDir: func() (string, error) {
					return "/home/alex", nil
				},
			},
			wantPath: filepath.Join("/state", "ding-ding", "logs"),
		},
		{
			name: "linux falls back to local state dir",
			opts: logDirResolverOptions{
				GOOS: "linux",
				getenv: func(string) string {
					return ""
				},
				userHomeDir: func() (string, error) {
					return "/home/alex", nil
				},
			},
			wantPath: filepath.Join("/home/alex", ".local", "state", "ding-ding", "logs"),
		},
		{
			name: "windows uses local app data when set",
			opts: logDirResolverOptions{
				GOOS: "windows",
				getenv: func(key string) string {
					if key == "LOCALAPPDATA" {
						return `C:\Users\alex\AppData\Local`
					}
					return ""
				},
				userCacheDir: func() (string, error) {
					return `C:\cache`, nil
				},
			},
			wantPath: filepath.Join(`C:\Users\alex\AppData\Local`, "ding-ding", "Logs"),
		},
		{
			name: "windows falls back to user cache dir",
			opts: logDirResolverOptions{
				GOOS: "windows",
				getenv: func(string) string {
					return ""
				},
				userCacheDir: func() (string, error) {
					return `C:\Users\alex\AppData\Local\Cache`, nil
				},
			},
			wantPath: filepath.Join(`C:\Users\alex\AppData\Local\Cache`, "ding-ding", "Logs"),
		},
		{
			name: "resolver falls back to legacy logs on lookup failure",
			opts: logDirResolverOptions{
				GOOS: "darwin",
				userHomeDir: func() (string, error) {
					return "", errors.New("boom")
				},
			},
			wantPath: legacyLogDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveDefaultLogDir(tt.opts)
			if got != tt.wantPath {
				t.Fatalf("resolveDefaultLogDir() = %q, want %q", got, tt.wantPath)
			}
		})
	}
}
