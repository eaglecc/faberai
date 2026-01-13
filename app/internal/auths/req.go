package auths

type RegisterReq struct {
	Username string `json:"username" binding:"required" validate:"required"`
	Password string `json:"password" binding:"required" validate:"required"`
	Email    string `json:"email" binding:"required" validate:"required,email"`
}

type VerifyEmailReq struct {
	Token string `json:"token" form:"token" binding:"required" validate:"required"`
}

type LoginReq struct {
	Username string `json:"username" binding:"required" validate:"required"`
	Password string `json:"password" binding:"required" validate:"required"`
}

type RefreshTokenReq struct {
	RefreshToken string `json:"refreshtoken" form:"refreshtoken" binding:"required" validate:"required"`
}

type ForgotPasswordReq struct {
	Email string `json:"email" binding:"required" validate:"required,email"`
}

type VerifyCodeReq struct {
	Email string `json:"email" binding:"required" validate:"required,email"`
	Code  string `json:"code" binding:"required" validate:"required,len=6"`
}

type ResetPasswordReq struct {
	Email       string `json:"email" binding:"required" validate:"required,email"`
	Token       string `json:"token" binding:"required" validate:"required,len=6"`
	NewPassword string `json:"newPassword" binding:"required" validate:"required,min=6"`
}
