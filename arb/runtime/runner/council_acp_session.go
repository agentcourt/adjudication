package runner

import (
	"errors"
	"fmt"
	"sort"

	"adjudication/common/acp"
)

type acpPersistentSession struct {
	client       *acp.Client
	sessionPath  string
	workspaceDir string
	cleanup      func() error
}

func (s *acpPersistentSession) Close() error {
	if s == nil {
		return nil
	}
	var err error
	if s.client != nil {
		err = errors.Join(err, s.client.Close())
	}
	if s.cleanup != nil {
		err = errors.Join(err, s.cleanup())
	}
	return err
}

func (rc *runContext) closeACPSessions() error {
	if len(rc.acpSessions) == 0 {
		return nil
	}
	roleNames := make([]string, 0, len(rc.acpSessions))
	for role := range rc.acpSessions {
		roleNames = append(roleNames, role)
	}
	sort.Strings(roleNames)
	var err error
	for _, role := range roleNames {
		session := rc.acpSessions[role]
		delete(rc.acpSessions, role)
		if closeErr := session.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close ACP session role=%s: %w", role, closeErr))
		}
	}
	return err
}
