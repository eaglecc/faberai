package agents

import (
	"app/shared"
	"common/biz"
	"context"
	"core/ai"
	"core/ai/mcps"
	"core/ai/tools"
	"encoding/json"
	"errors"
	"fmt"
	"model"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/supervisor"
	aiModel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/ollama/api"
	"github.com/google/uuid"
	"github.com/mszlu521/thunder/ai/einos"
	"github.com/mszlu521/thunder/database"
	"github.com/mszlu521/thunder/errs"
	"github.com/mszlu521/thunder/event"
	"github.com/mszlu521/thunder/logs"
)

type Service struct {
	repo repository
}

func NewService() *Service {
	return &Service{
		repo: NewModels(database.GetPostgresDB().GormDB),
	}
}

func (s *Service) createAgent(parent context.Context, req *CreateAgentRequest, userId uuid.UUID) (*model.Agent, error) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second) // 子上下文，不能超过10s
	defer cancel()
	agent := &model.Agent{
		BaseModel: model.BaseModel{
			ID: uuid.New(),
		},
		Name:            req.Name,
		Description:     req.Description,
		Status:          req.Status,
		CreatorID:       userId,
		SystemPrompt:    "",
		ModelProvider:   "",
		ModelName:       "",
		ModelParameters: model.JSON{},
		Version:         0,
		Visibility:      model.Private,
		InvocationCount: 0,
		OpeningDialogue: "",
	}
	err := s.repo.createAgent(ctx, agent)
	if err != nil {
		logs.Errorf("create agent error: %v", err)
		return nil, errs.DBError
	}
	return agent, nil
}

func (s *Service) listAgents(parent context.Context, req SearchRequest, userId uuid.UUID) (*ListAgentResponse, error) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	filter := AgentFilter{
		Name:   req.Name,
		Status: req.Status,
		Limit:  req.PageSize,
		Offset: (req.Page - 1) * req.PageSize,
	}
	agents, total, err := s.repo.listAgents(ctx, filter, userId)
	if err != nil {
		logs.Errorf("list agents error: %v", err)
		return nil, errs.DBError
	}
	return &ListAgentResponse{
		Agents: agents,
		Total:  total,
	}, nil
}

func (s *Service) getAgent(ctx context.Context, userId uuid.UUID, id uuid.UUID) (*model.Agent, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	agent, err := s.repo.getAgentById(ctx, userId, id)
	if err != nil {
		logs.Errorf("get agent error: %v", err)
		return nil, errs.DBError
	}
	if agent == nil {
		return nil, biz.ErrAgentNotFound
	}
	return agent, err
}

func (s *Service) updateAgent(ctx context.Context, userId uuid.UUID, req *UpdateAgentRequest) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// 先查询Id是否存在
	agent, err := s.repo.getAgentById(ctx, userId, req.Id)
	if err != nil {
		return nil, biz.ErrAgentNotFound
	}

	if req.Name != "" {
		agent.Name = req.Name
	}

	if req.Description != "" {
		agent.Description = req.Description
	}

	if req.Status != "" {
		agent.Status = req.Status
	}
	if req.ModelProvider != "" {
		agent.ModelProvider = req.ModelProvider
	}
	if req.ModelName != "" {
		agent.ModelName = req.ModelName
	}
	if req.ModelParameters != nil {
		agent.ModelParameters = req.ModelParameters
	}
	if req.SystemPrompt != "" {
		agent.SystemPrompt = req.SystemPrompt
	}
	if req.OpeningDialogue != "" {
		agent.OpeningDialogue = req.OpeningDialogue
	}
	if err := s.repo.updateAgent(ctx, agent); err != nil {
		return nil, errs.DBError
	}
	return nil, err

}

func (s *Service) agentMessageStream(ctx context.Context, userID uuid.UUID, req AgentMessageReq) (<-chan string, <-chan error) {
	dataChan := make(chan string, 100)
	errorChan := make(chan error, 10)
	go func() {
		//增加 defer 用于 recover。
		defer func() {
			if r := recover(); r != nil {
				logs.Errorf("Panic in agentMessageStream: %v", r)
				select {
				case errorChan <- errors.New("internal server error"):
				case <-ctx.Done():
					logs.Warn("发送取消：Context Done")
				}
			}
			// 确保通道关闭
			close(dataChan)
			close(errorChan)
		}()

		// 获取Agent
		agent, err := s.repo.getAgentById(ctx, userID, req.AgentId)
		if err != nil {
			s.sendError(ctx, errorChan, err)
			return
		}

		// 使用eino 框架中的 adk 来进行agent开发，这里需要创建一个主agent，支持多智能体协同工作
		mainAgent, err := s.buildMainAgent(ctx, agent, req.Message, dataChan)
		if err != nil {
			s.sendError(ctx, errorChan, err)
			return
		}
		// 构建supervisorAgent
		supervisorAgent, err := supervisor.New(ctx, &supervisor.Config{
			Supervisor: mainAgent,
			SubAgents:  []adk.Agent{},
		})
		if err != nil {
			s.sendError(ctx, errorChan, err)
			return
		}
		// 创建 runner
		runner := adk.NewRunner(ctx, adk.RunnerConfig{
			Agent:           supervisorAgent,
			EnableStreaming: true,
		})
		iter := runner.Query(ctx, req.Message)
		for {
			events, ok := iter.Next()
			if !ok {
				break
			}

			// 检查context是否已取消
			select {
			case <-ctx.Done():
				logs.Info("Stop generation: context canceled by client")
				return
			default:
			}

			if events.Err != nil {
				s.sendData(ctx, dataChan, ai.BuildErrMessage(events.AgentName, events.Err.Error()))
				return
			}
			// 处理输出
			if events.Output != nil && events.Output.MessageOutput != nil {
				msg, err := events.Output.MessageOutput.GetMessage()
				if err != nil {
					logs.Errorf("Error: failed to get message: %v\n", err)
					s.sendError(ctx, errorChan, err)
					return
				}
				if msg.Content == "" && msg.ReasoningContent == "" { // 是否有内容
					continue
				}
				if msg.ReasoningContent != "" {
					//思考内容
					s.sendData(ctx, dataChan, ai.BuildReasoningMessage(events.AgentName, msg.ToolName, msg.ReasoningContent))
				}
				println("Agent[" + events.AgentName + "]:\n" + msg.Content + "\n===========")
				println("Agent[" + events.AgentName + "] ToolName:\n" + msg.ToolName + "\n===========")

				if msg.Content != "" {
					// 可以在这里打印日志，但不要用 println (非并发安全且无法分级)
					// logs.Infof(...)
					s.sendData(ctx, dataChan, ai.BuildContentMessage(events.AgentName, msg.ToolName, msg.Content))
				}
			}
		}
	}()
	return dataChan, errorChan
}

func (s *Service) sendError(ctx context.Context, errorChan chan error, err error) {
	select {
	case <-ctx.Done():
		logs.Warn("发送取消：Context Done")
	case errorChan <- err:
	}
}

func (s *Service) sendData(ctx context.Context, dataChan chan string, data string) {
	select {
	case <-ctx.Done():
		logs.Warn("发送取消：Context Done")
	case dataChan <- data:
	}
}

// 创建主agent
func (s *Service) buildMainAgent(ctx context.Context, agent *model.Agent, message string, dataChan chan string) (adk.Agent, error) {
	// 1. 先获取当前agent的模型配置信息
	providerConfig, err := s.getProviderConfig(ctx, model.LLMTypeChat, agent.ModelProvider, agent.ModelName)
	if err != nil {
		return nil, err
	}
	if providerConfig == nil {
		return nil, biz.ProviderConfigNotFound
	}
	// 构建 chatmodel，这里需要调用llms包中的服务，所以需要定义,调用event事件
	chatModel, err := s.buildToolCallingChatModel(ctx, agent, providerConfig)
	if err != nil {
		logs.Error("Failed to build tool calling chat model", "err", err)
		return nil, err
	}
	var allTools []tool.BaseTool
	allTools = append(allTools, s.buildTools(agent)...)
	// 构建系统提示词 adk.NewChatModelAgent 实现了ReAct模式，能调用工具，多智能体协作
	//systemPrompt := fmt.Sprintf(ai.BASE_ADK_TEMPLATE, agentInfo.SystemPrompt, ragContext)
	modelAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Model:       chatModel,
		Description: agent.Description,
		Name:        agent.Name,
		Instruction: agent.SystemPrompt, // 基础提示词
		// GenModelInput 是发送给 大模型前做的处理
		GenModelInput: func(ctx context.Context, instruction string, input *adk.AgentInput) ([]adk.Message, error) {
			template := prompt.FromMessages(schema.FString,
				schema.SystemMessage(ai.BaseSystemPrompt),
			)
			messages, err := template.Format(ctx, map[string]any{
				"role":       agent.SystemPrompt,
				"toolsInfo":  s.formatToolsInfo(allTools),
				"agentsInfo": "",
				"ragContext": "",
			})
			if err != nil {
				logs.Error("Failed to format template", "err", err)
				return nil, err
			}
			messages = append(messages, input.Messages...)
			return messages, nil // messages 是最终给模型输入的内容
		},
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: allTools,
			},
		},
	})
	if err != nil {
		logs.Error("Failed to create chat model agent", "err", err)
		return nil, err
	}

	return modelAgent, nil
}

func (s *Service) getProviderConfig(ctx context.Context, llmType model.LLMType, provider string, name string) (*model.ProviderConfig, error) {
	// 这里需要调用llms包中的服务，所以需要定义,调用event事件
	trigger, err := event.Trigger("getProviderConfigByProvider", &shared.GetProviderConfigRequest{
		LLMType:   llmType,
		Provider:  provider,
		ModelName: name,
	})
	if err != nil {
		logs.Errorf("getProviderConfigByProvider error: %v", err)
		return nil, err
	}
	response := trigger.(*model.ProviderConfig)
	if response.Provider == "" {
		return nil, biz.ProviderConfigNotFound
	}
	return response, nil

}

func (s *Service) buildToolCallingChatModel(ctx context.Context, agentInfo *model.Agent, config *model.ProviderConfig) (aiModel.ToolCallingChatModel, error) {
	var chatModel aiModel.ToolCallingChatModel
	var err error
	modelParams := agentInfo.ModelParameters.ToModelParams()
	temperature := float32(modelParams.Temperature)
	topP := float32(modelParams.TopP)
	maxTokens := modelParams.MaxTokens

	// 打印配置信息以便调试
	logs.Infof("Building chat model - Provider: %s, BaseURL: %s, Model: %s", config.Provider, config.APIBase, agentInfo.ModelName)

	if config.Provider == model.OllamaProvider {
		// 创建聊天模型
		chatModel, err = ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
			BaseURL: config.APIBase,
			Model:   agentInfo.ModelName,
			Options: &api.Options{
				Temperature: temperature,
				TopP:        topP,
				Runner: api.Runner{
					NumCtx: maxTokens,
				},
			},
		})
	} else if config.Provider == model.QwenProvider {
		chatModel, err = qwen.NewChatModel(ctx, &qwen.ChatModelConfig{
			BaseURL:     config.APIBase,
			APIKey:      config.APIKey,
			Model:       agentInfo.ModelName,
			MaxTokens:   &maxTokens,
			Temperature: &temperature,
			TopP:        &topP,
		})
	} else {
		chatModel, err = openai.NewChatModel(ctx, &openai.ChatModelConfig{
			BaseURL:             config.APIBase,
			APIKey:              config.APIKey,
			Model:               agentInfo.ModelName,
			MaxCompletionTokens: &maxTokens,
			Temperature:         &temperature,
			TopP:                &topP,
		})
	}
	if err != nil {
		logs.Error("Failed to create chat model", "err", err)
		return nil, err
	}
	return chatModel, nil

}

func (s *Service) updateAgentTool(ctx context.Context, userID uuid.UUID, agentId uuid.UUID, req UpdateAgentToolReq) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	//先检查agent是否存在
	agent, err := s.repo.getAgentById(ctx, userID, agentId)
	if err != nil {
		return nil, errs.DBError
	}
	if agent == nil {
		return nil, biz.ErrAgentNotFound
	}
	if len(req.Tools) <= 0 {
		return nil, biz.ErrToolNotExisted
	}
	// todo: 可以把删除和新增放在一个事务中
	//先删除agent现有关联的工具
	err = s.repo.deleteAgentTools(ctx, agentId)
	if err != nil {
		return nil, errs.DBError
	}
	//创建新的关联记录
	var agentTools []*model.AgentTool
	var toolIds []uuid.UUID
	for _, v := range req.Tools {
		toolIds = append(toolIds, v.ID)
	}
	//获取到工具的ID，去工具表查询出对应的工具信息
	toolsList, err := s.getToolsByIds(toolIds)
	for _, t := range toolsList {
		agentTools = append(agentTools, &model.AgentTool{
			AgentID:   agentId,
			ToolID:    t.ID,
			Status:    model.Enabled,
			CreatedAt: time.Now(),
		})
	}
	//批量插入
	err = s.repo.createAgentTools(ctx, agentTools)
	if err != nil {
		logs.Errorf("批量插入agent_tools失败: %v", err)
		return nil, errs.DBError
	}
	return agentTools, nil
}

func (s *Service) getToolsByIds(ids []uuid.UUID) ([]*model.Tool, error) {
	//event 获取工具信息
	trigger, err := event.Trigger("getToolsByIds", &shared.GetToolsByIdsRequest{
		Ids: ids,
	})
	return trigger.([]*model.Tool), err
}

func (s *Service) buildTools(agent *model.Agent) []tool.BaseTool {
	var agentTools []tool.BaseTool
	for _, v := range agent.Tools {
		// 工具类型又system和mcp两种
		switch v.ToolType {
		case model.McpToolType:
			// 查询出MCP工具列表，转为Eino中的BaseTool
			mcpConfigs := einos.McpConfig{
				BaseUrl: v.McpConfig.Url,
				Token:   v.McpConfig.CredentialType,
				Name:    "FaberAI",
				Version: "1.0.0",
			}
			baseTools, err := mcps.GetEinoBaseTools(context.Background(), &mcpConfigs)
			if err != nil {
				logs.Warnf("获取MCP工具列表时出错: %v", err)
				continue
			}
			agentTools = append(agentTools, baseTools...)
		case model.SystemToolType:
			// 根据名称获取工具
			systemTool := s.loadSystemTool(v.Name)
			if systemTool == nil {
				logs.Warnf("加载系统工具时，找不到工具: %s", v.Name)
				continue
			}
			agentTools = append(agentTools, systemTool)
		default:
			logs.Warnf("Unknown tool type: %s", v.ToolType)
		}
	}
	return agentTools
}

func (s *Service) loadSystemTool(name string) tool.BaseTool {
	return tools.FindTool(name)
}

func (s *Service) formatToolsInfo(allTools []tool.BaseTool) string {
	var builder strings.Builder
	builder.WriteString("【可用工具列表】: \n")
	for _, tool := range allTools {
		info, _ := tool.Info(context.Background())
		builder.WriteString(fmt.Sprintf("名称: `%s`\n", info.Name))
		builder.WriteString(fmt.Sprintf("描述: `%s`\n", info.Desc))
		// 参数要转为JSON
		marshal, _ := json.Marshal(info.ParamsOneOf)
		builder.WriteString(fmt.Sprintf("参数: `%s`\n", string(marshal)))
	}
	return builder.String()
}
