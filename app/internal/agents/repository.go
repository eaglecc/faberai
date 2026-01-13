package agents

import (
	"context"
	"model"

	"github.com/google/uuid"
)

type repository interface {
	createAgent(ctx context.Context, agent *model.Agent) error
	listAgents(ctx context.Context, filter AgentFilter, userId uuid.UUID) ([]*model.Agent, int64, error)
	getAgentById(ctx context.Context, userId uuid.UUID, id uuid.UUID) (*model.Agent, error)
	updateAgent(ctx context.Context, agent *model.Agent) error
}
