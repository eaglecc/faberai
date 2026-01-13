package llms

import (
	"context"
	"model"
	"time"

	"github.com/google/uuid"
	"github.com/mszlu521/thunder/database"
	"github.com/mszlu521/thunder/errs"
	"github.com/mszlu521/thunder/logs"
)

type service struct {
	repo repository
}

func NewService() *service {
	return &service{repo: NewModels(database.GetPostgresDB().GormDB)}
}

// CreateProviderConfig 创建厂商配置
func (s *service) CreateProviderConfig(ctx context.Context, userID uuid.UUID, req CreateProviderConfigRequest) (*CreateProviderConfigResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	config := &model.ProviderConfig{
		BaseModel: model.BaseModel{
			ID: uuid.New(),
		},
		Name:        req.Name,
		UserID:      userID,
		Provider:    req.Provider,
		Description: req.Description,
		APIKey:      req.APIKey,
		APIBase:     req.APIBase,
		Status:      model.LLMStatus(req.Status),
	}

	err := s.repo.createProviderConfig(ctx, config)
	if err != nil {
		logs.Errorf("create provider config error: %v", err)
		return nil, errs.DBError
	}

	return &CreateProviderConfigResponse{
		ID: config.ID,
	}, nil
}

// ListProviderConfigs 查询厂商配置列表
func (s *service) ListProviderConfigs(ctx context.Context, userID uuid.UUID) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	configs, total, err := s.repo.listProviderConfigs(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &ListProviderConfigsResponse{
		ProviderConfigs: configs,
		Total:           total,
	}, nil
}

// CreateLLM 创建模型
func (s *service) CreateLLM(ctx context.Context, userID uuid.UUID, req CreateLLMRequest) (*CreateLLMResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	llm := &model.LLM{
		UserID:           userID,
		Name:             req.Name,
		Description:      req.Description,
		ProviderConfigID: req.ProviderConfigID,
		ModelName:        req.ModelName,
		Config:           req.Config,
		ModelType:        model.LLMType(req.ModelType),
		Status:           model.LLMStatus(req.Status),
	}

	err := s.repo.createLLM(ctx, llm)
	if err != nil {
		return nil, err
	}

	return &CreateLLMResponse{
		ID: llm.ID,
	}, nil
}

func (s *service) ListLLMs(ctx context.Context, userId uuid.UUID, req ListLLMsRequest) (*ListLLMsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	filter := LLMFilter{
		ModelType: req.ModelType,
	}

	llms, total, err := s.repo.listLLMS(ctx, userId, filter)
	if err != nil {
		return nil, err
	}

	return &ListLLMsResponse{
		LLMs:  llms,
		Total: total,
	}, nil
}
