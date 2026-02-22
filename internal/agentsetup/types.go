package agentsetup

import (
	"fmt"
	"strings"
)

type Agent string

const (
	AgentClaude   Agent = "claude"
	AgentOpenCode Agent = "opencode"
)

type Scope string

const (
	ScopeProject Scope = "project"
	ScopeGlobal  Scope = "global"
)

type Mode string

const (
	ModeCLI    Mode = "cli"
	ModeServer Mode = "server"
)

type ChangeStatus string

const (
	StatusCreated   ChangeStatus = "created"
	StatusUpdated   ChangeStatus = "updated"
	StatusUnchanged ChangeStatus = "unchanged"
)

type Options struct {
	Agent   Agent
	Scope   Scope
	Mode    Mode
	CWD     string
	HomeDir string
}

type Result struct {
	Path   string
	Status ChangeStatus
}

func ParseAgent(value string) (Agent, error) {
	switch Agent(strings.ToLower(strings.TrimSpace(value))) {
	case AgentClaude:
		return AgentClaude, nil
	case AgentOpenCode:
		return AgentOpenCode, nil
	default:
		return "", fmt.Errorf("invalid agent %q (supported: claude, opencode)", value)
	}
}

func ParseScope(value string) (Scope, error) {
	switch Scope(strings.ToLower(strings.TrimSpace(value))) {
	case ScopeProject:
		return ScopeProject, nil
	case ScopeGlobal:
		return ScopeGlobal, nil
	default:
		return "", fmt.Errorf("invalid scope %q (supported: project, global)", value)
	}
}

func ParseMode(value string) (Mode, error) {
	switch Mode(strings.ToLower(strings.TrimSpace(value))) {
	case ModeCLI:
		return ModeCLI, nil
	case ModeServer:
		return ModeServer, nil
	default:
		return "", fmt.Errorf("invalid mode %q (supported: cli, server)", value)
	}
}

func (o Options) Validate() error {
	if _, err := ParseAgent(string(o.Agent)); err != nil {
		return err
	}
	if _, err := ParseScope(string(o.Scope)); err != nil {
		return err
	}
	if _, err := ParseMode(string(o.Mode)); err != nil {
		return err
	}
	return nil
}
