package agentsetup

import "fmt"

func Upsert(opts Options) (Result, error) {
	agent, err := ParseAgent(string(opts.Agent))
	if err != nil {
		return Result{}, err
	}
	scope, err := ParseScope(string(opts.Scope))
	if err != nil {
		return Result{}, err
	}
	mode, err := ParseMode(string(opts.Mode))
	if err != nil {
		return Result{}, err
	}
	opts.Agent = agent
	opts.Scope = scope
	opts.Mode = mode

	path, err := ResolveTargetPath(opts)
	if err != nil {
		return Result{}, err
	}

	var status ChangeStatus
	switch opts.Agent {
	case AgentClaude:
		status, err = upsertClaudeSettings(path, opts.Mode)
	case AgentOpenCode:
		status, err = upsertOpenCodePlugin(path, opts.Mode)
	default:
		return Result{}, fmt.Errorf("unsupported agent %q", opts.Agent)
	}
	if err != nil {
		return Result{}, err
	}

	return Result{Path: path, Status: status}, nil
}
