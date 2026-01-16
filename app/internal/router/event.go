package router

import (
	"app/internal/llms"
	"app/internal/tools"

	"github.com/mszlu521/thunder/event"
)

type Event struct {
}

// 注册事件路由
func (*Event) Register() {
	// 注册事件相关的路由
	llmService := llms.NewPublicService()
	event.Register("getProviderConfigByProvider", llmService.GetProviderConfig)
	//event.Register("getEmbeddingConfig", llmService.GetEmbeddingConfig)
	toolService := tools.NewPublicService()
	event.Register("getToolsByIds", toolService.GetToolsByIds)
	//knowledgeService := knowledges.NewPublicService()
	//event.Register("getKnowledgeBase", knowledgeService.GetKnowledgeBase)
	//event.Register("searchKnowledgeBase", knowledgeService.SearchKnowledgeBase)
}
