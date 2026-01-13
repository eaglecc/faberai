package router

import (
	"app/internal/llms"

	"github.com/mszlu521/thunder/event"
)

type Event struct {
}

// 注册事件路由
func (*Event) Register() {
	llmService := llms.NewPublicService()
	event.Register("getProviderConfigByProvider", llmService.GetProviderConfig)
}
