package llms

import (
	"context"
	"model"

	"github.com/google/uuid"
	"github.com/mszlu521/thunder/gorms"
	"gorm.io/gorm"
)

type models struct {
	db *gorm.DB
}

func NewModels(db *gorm.DB) *models {
	return &models{
		db: db,
	}
}

type LLMFilter struct {
	ModelType model.LLMType
	Limit     int
	offset    int
}

func (r *models) createProviderConfig(ctx context.Context, config *model.ProviderConfig) error {
	return r.db.WithContext(ctx).Create(config).Error
}

// ListProviderConfigs 根据过滤条件查询厂商配置列表
func (r *models) listProviderConfigs(ctx context.Context, userId uuid.UUID) ([]*model.ProviderConfig, int64, error) {
	var providerConfigs []*model.ProviderConfig
	var total int64
	query := r.db.WithContext(ctx).Model(model.ProviderConfig{})
	return providerConfigs, total, query.Where("user_id = ?", userId).Find(&providerConfigs).Count(&total).Error
}

func (m *models) createLLM(ctx context.Context, llm *model.LLM) error {
	return m.db.WithContext(ctx).Create(llm).Error
}

func (m *models) listLLMS(ctx context.Context, userId uuid.UUID, filter LLMFilter) ([]*model.LLM, int64, error) {
	var llms []*model.LLM
	var total int64
	query := m.db.WithContext(ctx).Model(model.LLM{})
	if filter.ModelType != "" {
		query = query.Where("model_type = ?", filter.ModelType)
	}
	if filter.Limit > 0 && filter.offset > 0 {
		query = query.Limit(filter.Limit).Offset(filter.offset)
	}
	return llms, total, query.Preload("ProviderConfig").Find(&llms).Count(&total).Error
}

func (m *models) getProviderConfig(ctx context.Context, provider string) (*model.ProviderConfig, error) {
	var providerConfig model.ProviderConfig
	err := m.db.WithContext(ctx).Where("provider = ? ", provider).First(&providerConfig).Error
	if gorms.IsRecordNotFoundError(err) {
		return nil, nil
	}
	return &providerConfig, err
}
