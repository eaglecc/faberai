package llms

import (
	"context"
	"model"

	"github.com/google/uuid"
)

type repository interface {
	createProviderConfig(ctx context.Context, config *model.ProviderConfig) error
	listProviderConfigs(ctx context.Context, userId uuid.UUID) ([]*model.ProviderConfig, int64, error)
	createLLM(ctx context.Context, llm *model.LLM) error
	listLLMS(ctx context.Context, userId uuid.UUID, filter LLMFilter) ([]*model.LLM, int64, error)
	getProviderConfig(ctx context.Context, provider string) (*model.ProviderConfig, error)
}
