package router

import (
	"app/internal/tools"

	"github.com/gin-gonic/gin"
)

type ToolsRouter struct {
}

func (t *ToolsRouter) Register(r *gin.Engine) {
	toolHandler := tools.NewHandler()

	toolGroup := r.Group("/api/v1/tools")
	{
		toolGroup.POST("", toolHandler.CreateTool)
		toolGroup.PUT("/:id", toolHandler.UpdateTool)
		toolGroup.DELETE("/:id", toolHandler.DeleteTool)
		// toolGroup.GET("/:id", toolHandler.GetTool)
		// 添加GET方法的工具列表接口，以匹配前端定义
		toolGroup.GET("", toolHandler.ListTools)
		toolGroup.POST("/:id/test", toolHandler.TestTool)
		toolGroup.GET("/mcp/:mcpId/tools", toolHandler.GetMcpTools)
	}
}
