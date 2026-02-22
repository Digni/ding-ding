package agentsetup

import (
	"fmt"
	"os"
	"path/filepath"
)

var lookupWorkingDir = os.Getwd
var lookupUserHomeDir = os.UserHomeDir

func ResolveTargetPath(opts Options) (string, error) {
	if err := opts.Validate(); err != nil {
		return "", err
	}

	baseDir := opts.CWD
	if opts.Scope == ScopeGlobal {
		baseDir = opts.HomeDir
	}

	if baseDir == "" {
		var err error
		if opts.Scope == ScopeProject {
			baseDir, err = lookupWorkingDir()
			if err != nil {
				return "", fmt.Errorf("resolve project directory: %w", err)
			}
		} else {
			baseDir, err = lookupUserHomeDir()
			if err != nil {
				return "", fmt.Errorf("resolve home directory: %w", err)
			}
		}
	}

	switch opts.Agent {
	case AgentClaude:
		return filepath.Join(baseDir, ".claude", "settings.json"), nil
	case AgentOpenCode:
		if opts.Scope == ScopeProject {
			return filepath.Join(baseDir, ".opencode", "plugins", "ding-ding.ts"), nil
		}
		return filepath.Join(baseDir, ".config", "opencode", "plugins", "ding-ding.ts"), nil
	default:
		return "", fmt.Errorf("unsupported agent %q", opts.Agent)
	}
}
