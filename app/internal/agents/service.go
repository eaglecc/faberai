package agents

import (
	"app/shared"
	"common/biz"
	"context"
	"core/ai"
	"errors"
	"model"
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
				"toolsInfo":  "",
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
