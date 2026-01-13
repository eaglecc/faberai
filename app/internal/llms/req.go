package llms

import (
	"model"

	"github.com/google/uuid"
)

type CreateProviderConfigRequest struct {
	Name        string `json:"name" binding:"required"`     // 提供商名称
	Provider    string `json:"provider" binding:"required"` // 提供商标识
	Description string `json:"description"`                 // 描述
	APIKey      string `json:"apiKey"`                      // API密钥
	APIBase     string `json:"apiBase"`                     // API地址
	Status      string `json:"status"`                      // 状态
}

type CreateLLMRequest struct {
	Name             string          `json:"name" binding:"required"`             // 模型名称
	Description      string          `json:"description"`                         // 描述
	ProviderConfigID uuid.UUID       `json:"providerConfigId" binding:"required"` // 关联的厂商配置ID
	ModelName        string          `json:"modelName" binding:"required"`        // 模型标识
	ModelType        string          `json:"modelType"`                           // 模型类型
	Config           model.LLMConfig `json:"config"`                              // 其他关键配置
	Status           string          `json:"status"`                              // 状态
}

type ListLLMsRequest struct {
	ModelType model.LLMType `json:"modelType" form:"modelType"`
}
