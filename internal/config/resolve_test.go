package config

import (
	"testing"
)

func TestResolveConfigSource_PreferenceAndFallback(t *testing.T) {
	tests := []struct {
		name          string
		states        map[string]resolveFileState
		envPath       string
		wantType      SourceType
		wantPath      string
		wantReason    string
		wantErr       bool
		inspectErrFor string
	}{
		{
			name: "preferred wins when both preferred and legacy exist",
			states: map[string]resolveFileState{
				"preferred.yaml": resolveFilePresent,
				"legacy.yaml":    resolveFilePresent,
			},
			wantType:   SourceConfigFile,
			wantPath:   "preferred.yaml",
			wantReason: "selected preferred config path",
		},
		{
			name: "legacy fallback when preferred missing",
			states: map[string]resolveFileState{
				"preferred.yaml": resolveFileMissing,
				"legacy.yaml":    resolveFilePresent,
			},
			wantType:   SourceConfigFile,
			wantPath:   "legacy.yaml",
			wantReason: "fallback to legacy path because preferred config is missing",
		},
		{
			name: "legacy fallback when preferred unreadable",
			states: map[string]resolveFileState{
				"preferred.yaml": resolveFileUnreadable,
				"legacy.yaml":    resolveFilePresent,
			},
			wantType:   SourceConfigFile,
			wantPath:   "legacy.yaml",
			wantReason: "fallback to legacy path because preferred config is unreadable",
		},
		{
			name: "environment path wins when no config file is readable",
			states: map[string]resolveFileState{
				"preferred.yaml": resolveFileMissing,
				"legacy.yaml":    resolveFileMissing,
			},
			envPath:    "env.yaml",
			wantType:   SourceEnvironment,
			wantPath:   "env.yaml",
			wantReason: "selected by DING_DING_CONFIG",
		},
		{
			name: "defaults win when no source exists",
			states: map[string]resolveFileState{
				"preferred.yaml": resolveFileMissing,
				"legacy.yaml":    resolveFileMissing,
			},
			wantType:   SourceDefaults,
			wantPath:   "",
			wantReason: "no config file or environment path found",
		},
		{
			name:          "unexpected inspect error fails fast",
			states:        map[string]resolveFileState{"preferred.yaml": resolveFileMissing},
			inspectErrFor: "preferred.yaml",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := ResolveConfigSource(ResolveOptions{
				GOOS:          "darwin",
				EnvPath:       tt.envPath,
				PreferredPath: "preferred.yaml",
				LegacyPath:    "legacy.yaml",
				inspectFile: func(path string) (resolveFileState, error) {
					if path == tt.inspectErrFor {
						return resolveFileMissing, assertError("boom")
					}
					if state, ok := tt.states[path]; ok {
						return state, nil
					}
					return resolveFileMissing, nil
				},
			})

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if source.Type != tt.wantType {
				t.Fatalf("source type = %q, want %q", source.Type, tt.wantType)
			}

			if source.Path != tt.wantPath {
				t.Fatalf("source path = %q, want %q", source.Path, tt.wantPath)
			}

			if source.Reason != tt.wantReason {
				t.Fatalf("source reason = %q, want %q", source.Reason, tt.wantReason)
			}
		})
	}
}

func TestResolveConfigSource_RepeatedRunsStayDeterministic(t *testing.T) {
	states := map[string]resolveFileState{
		"preferred.yaml": resolveFilePresent,
		"legacy.yaml":    resolveFilePresent,
	}

	for i := 0; i < 10; i++ {
		source, err := ResolveConfigSource(ResolveOptions{
			GOOS:          "darwin",
			PreferredPath: "preferred.yaml",
			LegacyPath:    "legacy.yaml",
			inspectFile: func(path string) (resolveFileState, error) {
				return states[path], nil
			},
		})
		if err != nil {
			t.Fatalf("run %d: unexpected error: %v", i, err)
		}
		if source.Path != "preferred.yaml" {
			t.Fatalf("run %d: got %q, want preferred.yaml", i, source.Path)
		}
	}
}

type assertError string

func (e assertError) Error() string {
	return string(e)
}
