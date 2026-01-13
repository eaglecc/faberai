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
