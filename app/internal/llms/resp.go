package llms

import (
	"model"

	"github.com/google/uuid"
)

type CreateProviderConfigResponse struct {
	ID uuid.UUID `json:"id"`
}

type ListProviderConfigsResponse struct {
	ProviderConfigs []*model.ProviderConfig `json:"providerConfigs"`
	Total           int64                   `json:"total"`
}

type CreateLLMResponse struct {
	ID uuid.UUID `json:"id"`
}

type ListLLMsResponse struct {
	LLMs  []*model.LLM `json:"llms"`
	Total int64        `json:"total"`
}
