package agents

import (
	"model"

	"github.com/google/uuid"
)

type CreateAgentRequest struct {
	Name        string            `json:"name" `
	Description string            `json:"description"`
	Status      model.AgentStatus `json:"status"`
}

type SearchRequest struct {
	Name     string            `json:"name"`
	Status   model.AgentStatus `json:"status"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
}

type UpdateAgentRequest struct {
	Id              uuid.UUID         `json:"id" `
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Status          model.AgentStatus `json:"status"`
	SystemPrompt    string            `json:"systemPrompt"`
	ModelName       string            `json:"modelName"`
	ModelProvider   string            `json:"modelProvider"`
	ModelParameters model.JSON        `json:"modelParameters"`
	OpeningDialogue string            `json:"openingDialogue"`
}

type AgentMessageReq struct {
	AgentId   uuid.UUID `json:"agentId"`
	Message   string    `json:"message"`
	SessionId uuid.UUID `json:"sessionId"`
}

type UpdateAgentToolReq struct {
	Tools []ToolItem `json:"tools"`
}

type ToolItem struct {
	ID   uuid.UUID `json:"id"`
	Type string    `json:"type"`
}
