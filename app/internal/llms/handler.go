package llms

import (
	"github.com/gin-gonic/gin"
	"github.com/mszlu521/thunder/logs"
	"github.com/mszlu521/thunder/req"
	"github.com/mszlu521/thunder/res"
)

type Handler struct {
	service *service
}

func NewHandler() *Handler {
	return &Handler{service: NewService()}
}

func (h *Handler) CreateProviderConfig(c *gin.Context) {
	var createReq CreateProviderConfigRequest
	if err := req.JsonParam(c, &createReq); err != nil {
		return
	}
	userId, exist := req.GetUserIdUUID(c)
	if !exist {
		return
	}

	response, err := h.service.CreateProviderConfig(c.Request.Context(), userId, createReq)
	if err != nil {
		logs.Errorf("创建厂商配置失败: %v", err)
		res.Error(c, err)
		return
	}

	res.Success(c, response)
}

func (h *Handler) ListProviderConfigs(c *gin.Context) {
	userId, exist := req.GetUserIdUUID(c)
	if !exist {
		return
	}

	response, err := h.service.ListProviderConfigs(c.Request.Context(), userId)
	if err != nil {
		logs.Errorf("查询厂商配置列表失败: %v", err)
		res.Error(c, err)
		return
	}

	res.Success(c, response)
}

func (h *Handler) CreateLLM(c *gin.Context) {
	var createReq CreateLLMRequest
	if err := req.JsonParam(c, &createReq); err != nil {
		return
	}
	userId, exist := req.GetUserIdUUID(c)
	if !exist {
		return
	}

	response, err := h.service.CreateLLM(c.Request.Context(), userId, createReq)
	if err != nil {
		logs.Errorf("创建模型失败: %v", err)
		res.Error(c, err)
		return
	}

	res.Success(c, response)
}

func (h *Handler) ListLLMs(c *gin.Context) {
	var listReq ListLLMsRequest
	if err := req.QueryParam(c, &listReq); err != nil {
		return
	}
	userId, exist := req.GetUserIdUUID(c)
	if !exist {
		return
	}
	response, err := h.service.ListLLMs(c.Request.Context(), userId, listReq)
	if err != nil {
		logs.Errorf("查询模型列表失败: %v", err)
		res.Error(c, err)
		return
	}

	res.Success(c, response)
}
