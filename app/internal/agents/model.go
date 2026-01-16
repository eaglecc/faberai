package agents

import (
	"context"
	"model"
	"time"

	"github.com/google/uuid"
	"github.com/mszlu521/thunder/gorms"
	"gorm.io/gorm"
)

type models struct {
	db *gorm.DB
}

func NewModels(db *gorm.DB) *models {
	return &models{db: db}
}

// 过滤结构体，根据需求进行过滤
type AgentFilter struct {
	Name   string
	Status model.AgentStatus
	Limit  int
	Offset int
}

func (m *models) createAgent(ctx context.Context, agent *model.Agent) error {
	return m.db.WithContext(ctx).Create(agent).Error
}

func (m *models) listAgents(ctx context.Context, filter AgentFilter, userId uuid.UUID) ([]*model.Agent, int64, error) {
	var agents []*model.Agent
	var total int64
	query := m.db.WithContext(ctx).Model(&model.Agent{})
	query = query.Where("creator_id = ?", userId)
	if filter.Name != "" {
		query = query.Where("name LIKE ?", "%"+filter.Name+"%")
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}

	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	err := query.Find(&agents).Error
	if err != nil {
		return nil, 0, err
	}
	err = query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	return agents, total, nil
}

func (m *models) getAgentById(ctx context.Context, userId uuid.UUID, id uuid.UUID) (*model.Agent, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var agent model.Agent
	err := m.db.WithContext(ctx).Where("id = ? AND creator_id = ?", id, userId).First(&agent).Error
	if gorms.IsRecordNotFoundError(err) {
		return nil, nil
	}
	return &agent, err
}

func (m *models) updateAgent(ctx context.Context, agent *model.Agent) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return m.db.WithContext(ctx).Updates(agent).Error
}

func (m *models) deleteAgentTools(ctx context.Context, agentId uuid.UUID) error {
	return m.db.WithContext(ctx).Where("agent_id = ?", agentId).Delete(&model.AgentTool{}).Error
}

func (m *models) createAgentTools(ctx context.Context, tools []*model.AgentTool) error {
	return m.db.WithContext(ctx).CreateInBatches(tools, len(tools)).Error
}
