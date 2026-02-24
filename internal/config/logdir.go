package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const legacyLogDir = "logs"

type logDirResolverOptions struct {
	GOOS         string
	getenv       func(string) string
	userHomeDir  func() (string, error)
	userCacheDir func() (string, error)
}

func defaultLogDir() string {
	return resolveDefaultLogDir(logDirResolverOptions{})
}

func resolveDefaultLogDir(opts logDirResolverOptions) string {
	goos := strings.TrimSpace(opts.GOOS)
	if goos == "" {
		goos = runtime.GOOS
	}

	getenv := opts.getenv
	if getenv == nil {
		getenv = os.Getenv
	}

	userHomeDir := opts.userHomeDir
	if userHomeDir == nil {
		userHomeDir = os.UserHomeDir
	}

	userCacheDir := opts.userCacheDir
	if userCacheDir == nil {
		userCacheDir = os.UserCacheDir
	}

	switch goos {
	case "darwin":
		home, err := userHomeDir()
		if err != nil || strings.TrimSpace(home) == "" {
			return legacyLogDir
		}
		return filepath.Join(home, "Library", "Logs", "ding-ding")
	case "linux":
		if stateHome := strings.TrimSpace(getenv("XDG_STATE_HOME")); stateHome != "" {
			return filepath.Join(stateHome, "ding-ding", "logs")
		}

		home, err := userHomeDir()
		if err != nil || strings.TrimSpace(home) == "" {
			return legacyLogDir
		}
		return filepath.Join(home, ".local", "state", "ding-ding", "logs")
	case "windows":
		if localAppData := strings.TrimSpace(getenv("LOCALAPPDATA")); localAppData != "" {
			return filepath.Join(localAppData, "ding-ding", "Logs")
		}

		cacheDir, err := userCacheDir()
		if err != nil || strings.TrimSpace(cacheDir) == "" {
			return legacyLogDir
		}
		return filepath.Join(cacheDir, "ding-ding", "Logs")
	default:
		return legacyLogDir
	}
}

func normalizeLoggingDir(dir string) string {
	if isLegacyLogDir(dir) {
		return defaultLogDir()
	}
	return dir
}

func isLegacyLogDir(dir string) bool {
	trimmed := strings.TrimSpace(dir)
	if trimmed == "" || filepath.IsAbs(trimmed) {
		return false
	}

	return filepath.Clean(trimmed) == legacyLogDir
}
