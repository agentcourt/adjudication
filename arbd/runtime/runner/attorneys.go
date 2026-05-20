package runner

import (
	"fmt"
	"path/filepath"
	"strings"
)

const defaultRemoteSessionCwd = "/home/user"

func resolveAttorney(role string, cfg Config, complaintPath string) (AttorneyRunInfo, error) {
	role = strings.TrimSpace(role)
	base := AttorneyRoleConfig{
		Model:      cfg.AttorneyModel,
		ACPCommand: cfg.ACPCommand,
	}
	var roleModel string
	switch role {
	case "plaintiff":
		roleModel = strings.TrimSpace(cfg.PlaintiffAttorney.Model)
		base = mergeAttorneyRoleConfig(base, cfg.PlaintiffAttorney)
	case "defendant":
		roleModel = strings.TrimSpace(cfg.DefendantAttorney.Model)
		base = mergeAttorneyRoleConfig(base, cfg.DefendantAttorney)
	default:
		return AttorneyRunInfo{}, fmt.Errorf("unsupported attorney role %q", role)
	}
	sessionCwd := strings.TrimSpace(base.SessionCwd)
	endpoint := strings.TrimSpace(base.ACPEndpoint)
	if endpoint != "" {
		if roleModel != "" {
			return AttorneyRunInfo{}, fmt.Errorf("%s attorney model cannot be set with an ACP endpoint; the remote ACP attorney owns model selection", role)
		}
		if sessionCwd == "" {
			sessionCwd = defaultRemoteSessionCwd
		}
		return AttorneyRunInfo{
			Role:         role,
			ACPTransport: "tcp",
			ACPEndpoint:  endpoint,
			SessionCwd:   sessionCwd,
		}, nil
	}
	model := strings.TrimSpace(base.Model)
	if model == "" {
		model = DefaultAttorneyModel
	}
	spec, err := parseAttorneyModel(model)
	if err != nil {
		return AttorneyRunInfo{}, err
	}
	command := strings.TrimSpace(base.ACPCommand)
	if command == "" {
		return AttorneyRunInfo{}, fmt.Errorf("%s attorney ACP command is required", role)
	}
	if sessionCwd == "" {
		sessionCwd, err = filepath.Abs(filepath.Dir(complaintPath))
		if err != nil {
			return AttorneyRunInfo{}, fmt.Errorf("resolve %s attorney session cwd: %w", role, err)
		}
	}
	searchEnabled := spec.SearchRequested
	return AttorneyRunInfo{
		Role:          role,
		Model:         model,
		SearchEnabled: &searchEnabled,
		ACPTransport:  "stdio",
		ACPCommand:    command,
		SessionCwd:    sessionCwd,
	}, nil
}

func mergeAttorneyRoleConfig(base AttorneyRoleConfig, override AttorneyRoleConfig) AttorneyRoleConfig {
	if strings.TrimSpace(override.Model) != "" {
		base.Model = strings.TrimSpace(override.Model)
	}
	if strings.TrimSpace(override.ACPCommand) != "" {
		base.ACPCommand = strings.TrimSpace(override.ACPCommand)
	}
	if strings.TrimSpace(override.ACPEndpoint) != "" {
		base.ACPEndpoint = strings.TrimSpace(override.ACPEndpoint)
	}
	if strings.TrimSpace(override.SessionCwd) != "" {
		base.SessionCwd = strings.TrimSpace(override.SessionCwd)
	}
	return base
}

func attorneyRunInfos(cfg Config, complaintPath string) ([]AttorneyRunInfo, error) {
	plaintiff, err := resolveAttorney("plaintiff", cfg, complaintPath)
	if err != nil {
		return nil, err
	}
	defendant, err := resolveAttorney("defendant", cfg, complaintPath)
	if err != nil {
		return nil, err
	}
	return []AttorneyRunInfo{plaintiff, defendant}, nil
}
