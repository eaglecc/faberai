package agents

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mszlu521/thunder/logs"
	"github.com/mszlu521/thunder/req"
	"github.com/mszlu521/thunder/res"
)

type Handler struct {
	service *Service
}

func NewHandler() *Handler {
	return &Handler{
		service: NewService(),
	}
}

func (h *Handler) CreateAgent(c *gin.Context) {
	var createAgentReq CreateAgentRequest
	if err := req.JsonParam(c, &createAgentReq); err != nil {
		return
	}
	// 从上下文中获取用户信息
	userID, exists := req.GetUserIdUUID(c)
	if !exists {
		return
	}
	// 如果需要创建链路追踪，上下文要进行传递
	resp, err := h.service.createAgent(c.Request.Context(), &createAgentReq, userID)
	if err != nil {
		res.Error(c, err)
		return
	}

	res.Success(c, resp)
}

func (h Handler) ListAgents(c *gin.Context) {
	var searchReq SearchRequest
	if err := req.JsonParam(c, &searchReq); err != nil {
		return
	}
	userID, exists := req.GetUserIdUUID(c)
	if !exists {
		return
	}
	agents, err := h.service.listAgents(
		c.Request.Context(),
		searchReq,
		userID,
	)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, agents)
}

func (h *Handler) GetAgent(c *gin.Context) {
	var id uuid.UUID
	err := req.Path(c, "id", &id)
	if err != nil {
		return
	}

	// 从上下文中获取用户信息
	userID, exists := req.GetUserIdUUID(c)
	if !exists {
		return
	}

	agent, err := h.service.getAgent(c.Request.Context(), userID, id)
	if err != nil {
		res.Error(c, err)
		return
	}

	res.Success(c, agent)
}

func (h *Handler) UpdateAgent(c *gin.Context) {
	var updateReq UpdateAgentRequest
	if err := req.JsonParam(c, &updateReq); err != nil {
		return
	}
	// 从上下文中获取用户信息
	userID, exists := req.GetUserIdUUID(c)
	if !exists {
		return
	}

	_, err := h.service.updateAgent(c.Request.Context(), userID, &updateReq)
	if err != nil {
		res.Error(c, err)
		return
	}

	res.Success(c, nil)
}

func (h *Handler) AgentMessage(c *gin.Context) {
	var agentMessageReq AgentMessageReq
	if err := req.JsonParam(c, &agentMessageReq); err != nil {
		return
	}
	userId, exist := req.GetUserIdUUID(c)
	if !exist {
		return
	}
	// AI回答响应时间比较长，所以这里不能设限制，全局是10s超时，这里需要单独设置超时
	rc := http.NewResponseController(c.Writer)
	//  将当前请求的写入超时设置为零值（即无限制）
	// 这会覆盖全局 http.Server 的 WriteTimeout 设置
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		// 如果失败，记录日志，但通常不会失败
		logs.Warn("Failed to set write deadline", "err", err)
	}

	// SSE响应，需要设置SSE响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	//生产环境使用指定域名
	c.Header("Access-Control-Allow-Origin", "*")

	// 使用带取消功能的context
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// 这个接口是AI回答，返回两个chan，一个用于返回数据，一个用于返回错误
	// 调用大模型，需要放在协程中执行
	dataChan, errorChan := h.service.agentMessageStream(ctx, userId, agentMessageReq)
	// 创建一个心跳定时器，例如每 5 秒跳一次，防止一些防火墙拦截，导致连接中断
	heartbeat := time.NewTicker(5 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			logs.Warnf("context done, 客户端断开连接")
			return
		case <-heartbeat.C:
			// 发送心跳包，冒号开头，表示是心跳包
			_, err := c.Writer.Write([]byte(": keep-alive\n\n"))
			if err != nil {
				logs.Warnf("failed to write heartbeat: %v", err)
				cancel()
				return
			}
			// 需要使用flush 将内存中缓存的数据强制写入到io中
			c.Writer.Flush()
		case data, ok := <-dataChan:
			if !ok {
				// 消息处理完成，channel 被关闭了，消息结束
				// 按照SSE协议，消息结束需要发送 [DONE]
				_, err := c.Writer.Write([]byte("[DONE]\n"))
				if err != nil {
					logs.Warnf("failed to write done: %v", err)
				}
				c.Writer.Flush()
				return
			}
			// 正常消息包
			_, err := c.Writer.Write([]byte(fmt.Sprintf("data: %s\n\n", data)))
			if err != nil {
				logs.Warnf("failed to write data: %v", err)
				cancel()
				return
			}
			c.Writer.Flush()
		case err, ok := <-errorChan:
			if !ok {
				// error 的消息结束不处理，交给datachan去处理
				errorChan = nil
				return
			}
			// 错误消息包
			if err != nil {
				_, err := c.Writer.Write([]byte("error: [ERROR]" + err.Error() + "\n\n"))
				if err != nil {
					logs.Errorf("failed to write error: %v", err)
					cancel()
					return
				}
				c.Writer.Flush()
				return
			}
		}
	}
}
