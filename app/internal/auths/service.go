package auths

import (
	"common/biz"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"model"
	"net/smtp"
	"time"

	"github.com/google/uuid"
	"github.com/mszlu521/thunder/cache"
	"github.com/mszlu521/thunder/config"
	"github.com/mszlu521/thunder/database"
	"github.com/mszlu521/thunder/errs"
	"github.com/mszlu521/thunder/logs"
	"github.com/mszlu521/thunder/tools/jwt"
	"github.com/mszlu521/thunder/tools/randoms"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Service struct {
	repo  repository
	cache *cache.RedisCache
}

func NewService() *Service {
	return &Service{
		repo:  NewModel(database.GetPostgresDB().GormDB),
		cache: cache.NewRedisCache(),
	}
}

func (s *Service) register(req RegisterReq) (*RegisterResp, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	// 1. 检查用户名是否存在
	u, err := s.repo.findByUsername(ctx, req.Username)
	if err != nil {
		logs.Errorf("findByUsername err: %v", err)
		return nil, errs.DBError
	}
	if u != nil {
		return nil, biz.ErrUserNameExisted
	}
	// 2. 检查邮箱是否注册
	u, err = s.repo.findByEmail(ctx, req.Email)
	if err != nil {
		logs.Errorf("findByEmail err: %v", err)
		return nil, errs.DBError
	}
	if u != nil {
		return nil, biz.ErrEmailExisted
	}
	// 3. 对密码进行加密
	password, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logs.Errorf("GenerateFromPassword err: %v", err)
		return nil, biz.ErrPasswordFormat
	}
	// 4. 生成邮件用的token 用于激活邮件
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, errs.DBError
	}
	verifyToken := hex.EncodeToString(tokenBytes)
	userId := uuid.New()
	// 5. 存入redis中，用户激活邮件的时候验证
	tokenKey := fmt.Sprintf("verify_token:%s", verifyToken)
	err = s.cache.Set(tokenKey, userId.String(), 24*60*60)
	if err != nil {
		return nil, err
	}
	// 6. 存入数据库
	user := model.User{
		Id:            userId,
		Username:      req.Username,
		Password:      string(password),
		Avatar:        "default",
		Status:        model.UserStatusPending,
		LastLoginTime: time.Now(),
		CurrentPlan:   model.FreePlan,
		Email:         req.Email,
		EmailVerified: false,
	}
	err = s.repo.transaction(ctx, func(tx *gorm.DB) error { // 新增操作和发邮件放在一起作为一个事务
		// 创建用户
		if err := s.repo.saveUser(ctx, tx, &user); err != nil {
			logs.Errorf("saveUser err: %v", err)
			return err
		}
		// 发邮件
		if err = s.sendVerifyEmail(user.Email, user.Username, verifyToken); err != nil {
			logs.Errorf("sendVerifyEmail err: %v", err)
			return err
		}
		return nil
	})
	if err != nil {
		return nil, errs.DBError
	}
	return &RegisterResp{Message: "注册成功，请检查您的邮箱并点击验证链接完成注册"}, nil

}

func (s *Service) sendVerifyEmail(email string, username string, token string) error {
	// 加载邮件配置
	emailConfig := config.GetConfig().Email

	// If email is not configured, skip sending
	if emailConfig.Host == nil || emailConfig.Port == nil {
		logs.Warn("Email not configured, skipping verification email")
		return nil
	}

	// Email content
	subject := "请验证您的邮箱地址"
	verifyURL := fmt.Sprintf("%s/api/v1/auth/verify-email?token=%s", emailConfig.GetBaseURL(), token)
	body := fmt.Sprintf("尊敬的 %s，\n\n感谢您注册我们的服务！\n\n请点击以下链接验证您的邮箱地址：\n%s\n\n如果链接无法点击，请复制并粘贴到浏览器地址栏中。\n\n谢谢！\n", username, verifyURL)

	// Set up authentication information
	auth := smtp.PlainAuth("", emailConfig.GetUsername(), emailConfig.GetPassword(), emailConfig.GetHost())

	// Connect to the server, authenticate, and send the email
	to := []string{email}
	msg := []byte("To: " + email + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" +
		body + "\r\n")

	addr := fmt.Sprintf("%s:%d", emailConfig.GetHost(), emailConfig.GetPort())
	err := smtp.SendMail(addr, auth, emailConfig.GetFrom(), to, msg)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// 验证邮箱
func (s *Service) verifyEmail(token string) (any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 从Redis中获取用户ID
	redisCache := cache.NewRedisCache()
	tokenKey := fmt.Sprintf("verify_token:%s", token)
	userIdStr, err := redisCache.Get(tokenKey)
	if err != nil || userIdStr == "" {
		logs.Errorf("invalid or expired token: %v", err)
		return nil, biz.ErrTokenInvalid
	}
	// 删除已使用的令牌
	redisCache.Set(tokenKey, "", 1) // 设置1秒后过期

	// 根据用户ID查找用户
	userId, err := uuid.Parse(userIdStr)
	if err != nil {
		logs.Errorf("failed to parse user ID from token: %v", err)
		return nil, biz.ErrTokenInvalid
	}

	u, err := s.repo.findById(ctx, userId)
	if err != nil {
		logs.Errorf("find user by ID err:%v", err)
		return nil, errs.DBError
	}
	if u == nil {
		return nil, biz.ErrUserNotFound
	}
	if u.EmailVerified {
		// 用户已验证邮箱，直接返回成功
		return nil, nil
	}
	// 更新用户的邮箱验证状态
	u.EmailVerified = true
	u.Status = model.UserStatusNormal
	err = s.repo.transaction(ctx, func(tx *gorm.DB) error {
		return s.repo.updateUser(ctx, tx, u)
	})

	if err != nil {
		return nil, errs.DBError
	}
	return nil, nil
}

// 登录
func (s *Service) login(data LoginReq) (*LoginResp, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	// 1. 验证用户名
	u, err := s.repo.findByUsernameOrEmail(ctx, data.Username)
	if err != nil {
		logs.Errorf("findByUsernameOrEmail err: %v", err)
		return nil, errs.DBError
	}
	if u == nil {
		return nil, biz.ErrUserNotFound
	}
	if !u.EmailVerified {
		return nil, biz.ErrEmailNotVerified
	}
	// 2. 验证密码
	err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(data.Password))
	if err != nil {
		logs.Errorf("CompareHashAndPassword err: %v", err)
		return nil, biz.ErrPasswordInvalid
	}
	return s.token(u)
}

// 生成token和refreshtoken
func (s *Service) token(u *model.User) (*LoginResp, error) {
	expire := config.GetConfig().Jwt.GetExpire()
	refreshExpire := config.GetConfig().Jwt.GetRefresh()
	token, err := jwt.GenToken(u.Id.String(), u.Username, expire)
	if err != nil {
		logs.Errorf("GenToken err: %v", err)
		return nil, biz.ErrTokenGenerate
	}
	refreshToken, err := jwt.GenToken(u.Id.String(), u.Username, refreshExpire)
	if err != nil {
		logs.Errorf("GenToken err: %v", err)
		return nil, biz.ErrTokenGenerate
	}
	return &LoginResp{
		Expire:        time.Now().Add(expire).UnixMilli(),
		Message:       "登录成功",
		RefreshExpire: time.Now().Add(refreshExpire).UnixMilli(),
		RefreshToken:  refreshToken,
		Token:         token,
		UserInfo: &model.UserDTO{
			Id:            u.Id,
			Username:      u.Username,
			Avatar:        u.Avatar,
			Status:        u.Status,
			LastLoginTime: u.LastLoginTime,
			CurrentPlan:   u.CurrentPlan,
		},
	}, nil
}

// 刷新token
func (s *Service) refreshToken(token string) (*LoginResp, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	// 解析refreshToken
	claims, err := jwt.ParseToken(token)
	if err != nil {
		logs.Errorf("ParseToken err: %v", err)
		return nil, biz.ErrTokenInvalid
	}
	userIdStr := claims.UserId
	userId, err := uuid.Parse(userIdStr)
	if err != nil {
		logs.Errorf("ParseToken err: %v", err)
		return nil, biz.ErrTokenInvalid
	}
	u, err := s.repo.findById(ctx, userId)
	if err != nil {
		logs.Errorf("findById err: %v", err)
		return nil, errs.DBError
	}
	if u == nil {
		return nil, biz.ErrUserNotFound
	}
	// 重新生成token 和 refreshtoken
	return s.token(u)
}

func (s *Service) forgotPassword(req ForgotPasswordReq) error {
	// 检查邮箱是否存在
	u, err := s.repo.findByEmail(context.Background(), req.Email)
	if err != nil {
		logs.Errorf("find email err:%v", err)
		return errs.DBError
	}
	if u == nil {
		return nil
	}

	// 生成验证码
	code, err := randoms.Gen6Code()
	if err != nil {
		logs.Errorf("gen code err:%v", err)
		return errs.DBError
	}
	// 将验证码存储在Redis中，设置5分钟过期时间
	redisCache := cache.NewRedisCache()
	codeKey := fmt.Sprintf("forgot_password_code:%s", req.Email)
	if err := redisCache.Set(codeKey, code, 5*60); err != nil { // 5分钟
		logs.Errorf("failed to store forgot password code in Redis: %v", err)
		return errs.DBError
	}
	// 发送验证码邮件
	if err := s.sendForgotPasswordEmail(u.Email, u.Username, code); err != nil {
		logs.Errorf("send forgot password email err:%v", err)
		return errs.DBError
	}
	return nil
}

func (s *Service) sendForgotPasswordEmail(email, username, code string) error {
	// Get email configuration from config
	emailConfig := config.GetConfig().Email

	// If email is not configured, skip sending
	if emailConfig.Host == nil || emailConfig.Port == nil {
		logs.Warn("Email not configured, skipping forgot password email")
		return nil
	}

	// Email content
	subject := "您的验证码"
	body := fmt.Sprintf("尊敬的 %s，\n\n您正在重置密码，验证码是：%s\n\n验证码5分钟内有效，如非本人操作请忽略。\n\n谢谢！\n", username, code)

	// Set up authentication information
	auth := smtp.PlainAuth("", emailConfig.GetUsername(), emailConfig.GetPassword(), emailConfig.GetHost())

	// Connect to the server, authenticate, and send the email
	to := []string{email}
	msg := []byte("To: " + email + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" +
		body + "\r\n")

	addr := fmt.Sprintf("%s:%d", emailConfig.GetHost(), emailConfig.GetPort())
	err := smtp.SendMail(addr, auth, emailConfig.GetFrom(), to, msg)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func (s *Service) verifyCode(req VerifyCodeReq) (string, error) {
	// 从Redis中获取验证码
	redisCache := cache.NewRedisCache()
	codeKey := fmt.Sprintf("forgot_password_code:%s", req.Email)
	storedCode, err := redisCache.Get(codeKey)
	if err != nil || storedCode == "" {
		return "", biz.InvalidToken
	}

	// 验证验证码是否正确
	if storedCode != req.Code {
		return "", biz.InvalidToken
	}

	// 验证码正确，生成一个用于重置密码的临时令牌
	resetToken, err := s.generateResetToken(req.Email)
	if err != nil {
		return "", err
	}

	// 删除已使用的验证码
	redisCache.Set(codeKey, "", 1) // 设置1秒后过期

	return resetToken, nil
}

// 为重置密码功能添加生成重置令牌的方法
func (s *Service) generateResetToken(email string) (string, error) {
	// Generate reset token
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", errs.DBError
	}
	resetToken := hex.EncodeToString(tokenBytes)

	// Store the reset token in Redis with a 1-hour expiration
	redisCache := cache.NewRedisCache()
	tokenKey := fmt.Sprintf("reset_token:%s", resetToken)
	if err := redisCache.Set(tokenKey, email, 60*60); err != nil { // 1 hour
		logs.Errorf("failed to store reset token in Redis: %v", err)
		return "", errs.DBError
	}

	return resetToken, nil
}

func (s *Service) resetPassword(req ResetPasswordReq) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	// 验证重置令牌
	email, err := s.validateResetToken(req.Token)
	if err != nil {
		return biz.ErrCodeExpired
	}

	// 确保邮箱匹配
	if email != req.Email {
		return biz.ErrEmailNotMatch
	}

	// 查找用户
	user, err := s.repo.findByEmail(context.Background(), email)
	if err != nil {
		return biz.ErrUserNotFound
	}

	// 生成新密码的哈希值
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return biz.ErrPasswordProcess
	}

	// 更新密码
	user.Password = string(hashedPassword)
	err = s.repo.transaction(ctx, func(tx *gorm.DB) error {
		return s.repo.updateUser(context.Background(), tx, user)
	})
	if err != nil {
		return errs.DBError
	}

	return nil
}

func (s *Service) validateResetToken(token string) (string, error) {
	// 从Redis中获取邮箱
	redisCache := cache.NewRedisCache()
	tokenKey := fmt.Sprintf("reset_token:%s", token)
	email, err := redisCache.Get(tokenKey)
	if err != nil || email == "" {
		return "", biz.InvalidToken
	}

	// 删除已使用的令牌
	redisCache.Set(tokenKey, "", 1) // 设置1秒后过期

	return email, nil
}
