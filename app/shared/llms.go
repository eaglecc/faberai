package shared

import "model"

type GetProviderConfigRequest struct {
	LLMType   model.LLMType
	Provider  string
	ModelName string
}
