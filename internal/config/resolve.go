package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"runtime"
)

type resolveFileState int

const (
	resolveFilePresent resolveFileState = iota
	resolveFileMissing
	resolveFileUnreadable
)

type ResolveOptions struct {
	EnvPath       string
	GOOS          string
	PreferredPath string
	LegacyPath    string
	inspectFile   func(path string) (resolveFileState, error)
}

func inspectConfigFile(path string) (resolveFileState, error) {
	f, err := os.Open(path)
	if err == nil {
		_ = f.Close()
		return resolveFilePresent, nil
	}

	if errors.Is(err, fs.ErrNotExist) {
		return resolveFileMissing, nil
	}

	if errors.Is(err, fs.ErrPermission) {
		return resolveFileUnreadable, nil
	}

	return resolveFileMissing, fmt.Errorf("inspect config file %q: %w", path, err)
}

func resolveMacOSPaths(preferredOverride, legacyOverride string) (string, string, error) {
	preferred := preferredOverride
	if preferred == "" {
		path, err := ConfigPath()
		if err != nil {
			return "", "", err
		}
		preferred = path
	}

	legacy := legacyOverride
	if legacy == "" {
		path, err := LegacyDarwinConfigPath()
		if err != nil {
			return "", "", err
		}
		legacy = path
	}

	return preferred, legacy, nil
}

func resolveDefaultPath(preferredOverride string) (string, error) {
	if preferredOverride != "" {
		return preferredOverride, nil
	}

	path, err := ConfigPath()
	if err != nil {
		return "", err
	}

	return path, nil
}

// ResolveConfigSource returns a single deterministic config winner without parsing.
func ResolveConfigSource(opts ResolveOptions) (SourceSelection, error) {
	inspect := opts.inspectFile
	if inspect == nil {
		inspect = inspectConfigFile
	}

	goos := opts.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}

	if goos == "darwin" {
		preferred, legacy, err := resolveMacOSPaths(opts.PreferredPath, opts.LegacyPath)
		if err != nil {
			return SourceSelection{}, err
		}

		state, err := inspect(preferred)
		if err != nil {
			return SourceSelection{}, err
		}

		switch state {
		case resolveFilePresent:
			return SourceSelection{Type: SourceConfigFile, Path: preferred, Reason: "selected preferred config path"}, nil
		case resolveFileMissing, resolveFileUnreadable:
			legacyState, legacyErr := inspect(legacy)
			if legacyErr != nil {
				return SourceSelection{}, legacyErr
			}

			if legacyState == resolveFilePresent {
				reason := "fallback to legacy path because preferred config is missing"
				if state == resolveFileUnreadable {
					reason = "fallback to legacy path because preferred config is unreadable"
				}
				return SourceSelection{Type: SourceConfigFile, Path: legacy, Reason: reason}, nil
			}
		}
	} else {
		path, err := resolveDefaultPath(opts.PreferredPath)
		if err != nil {
			return SourceSelection{}, err
		}

		state, err := inspect(path)
		if err != nil {
			return SourceSelection{}, err
		}

		if state == resolveFilePresent {
			return SourceSelection{Type: SourceConfigFile, Path: path, Reason: "selected config path"}, nil
		}
	}

	if opts.EnvPath != "" {
		return SourceSelection{Type: SourceEnvironment, Path: opts.EnvPath, Reason: "selected by DING_DING_CONFIG"}, nil
	}

	return SourceSelection{Type: SourceDefaults, Reason: "no config file or environment path found"}, nil
}
