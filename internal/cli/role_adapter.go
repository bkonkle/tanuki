package cli

import (
	"github.com/bkonkle/tanuki/internal/agent"
	"github.com/bkonkle/tanuki/internal/role"
)

// roleManagerAdapter adapts role.Manager to agent.RoleManager interface.
type roleManagerAdapter struct {
	manager *role.FileManager
}

// newRoleManagerAdapter creates a new adapter.
func newRoleManagerAdapter(manager *role.FileManager) agent.RoleManager {
	return &roleManagerAdapter{manager: manager}
}

// GetRoleInfo implements agent.RoleManager.GetRoleInfo.
func (a *roleManagerAdapter) GetRoleInfo(name string) (*agent.RoleInfo, error) {
	role, err := a.manager.Get(name)
	if err != nil {
		return nil, err
	}

	return &agent.RoleInfo{
		Name:            role.Name,
		SystemPrompt:    role.SystemPrompt,
		AllowedTools:    role.AllowedTools,
		DisallowedTools: role.DisallowedTools,
		ContextFiles:    role.ContextFiles,
	}, nil
}
