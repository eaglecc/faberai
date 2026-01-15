package biz

import "github.com/mszlu521/thunder/errs"

var (
	ErrUserNameExisted  = errs.NewError(1001, "用户名已存在")
	ErrEmailExisted     = errs.NewError(1002, "邮箱已存在")
	ErrPasswordFormat   = errs.NewError(1003, "密码格式错误")
	ErrTokenInvalid     = errs.NewError(1004, "令牌失效")
	ErrUserNotFound     = errs.NewError(1005, "用户不存在")
	ErrEmailNotVerified = errs.NewError(1006, "邮箱未验证")
	ErrPasswordInvalid  = errs.NewError(1007, "密码错误")
	ErrTokenGenerate    = errs.NewError(1008, "令牌生成失败")
	InvalidToken        = errs.NewError(1009, "验证码无效")
	ErrEmailNotMatch    = errs.NewError(1010, "邮箱不匹配")
	ErrCodeExpired      = errs.NewError(1011, "验证码已过期")
	ErrPasswordProcess  = errs.NewError(1012, "密码处理失败")
	ErrResetPwd         = errs.NewError(1013, "更新密码失败")
)

var (
	ErrAgentNotFound       = errs.NewError(2001, "Agent不存在")
	ProviderConfigNotFound = errs.NewError(2002, "ProviderConfig不存在")
)

var (
	ErrToolNameExisted     = errs.NewError(3001, "工具名称已存在")
	ErrToolNotExisted      = errs.NewError(3002, "工具不存在")
	ErrMcpConfigNotExisted = errs.NewError(3003, "McpConfig不存在")
	ErrGetMcpTools         = errs.NewError(3004, "获取McpTools失败")
)
var (
	ErrKnowledgeBaseNotFound   = errs.NewError(4001, "知识库不存在")
	FileLoadError              = errs.NewError(4002, "文件加载错误")
	ErrDocumentNotFound        = errs.NewError(4003, "文档不存在")
	ErrEmbeddingConfigNotFound = errs.NewError(4004, "EmbeddingConfig不存在")
	ErrEmbedding               = errs.NewError(4005, "Embedding错误")
	ErrRetriever               = errs.NewError(4006, "Retriever错误")
)
