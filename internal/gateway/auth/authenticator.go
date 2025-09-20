package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// AuthStrategy 认证策略
type AuthStrategy string

const (
	NoAuth     AuthStrategy = "none"
	APIKey     AuthStrategy = "apikey"
	JWT        AuthStrategy = "jwt"
	BasicAuth  AuthStrategy = "basic"
	OAuth2     AuthStrategy = "oauth2"
)

// User 用户信息
type User struct {
	ID       string            `json:"id"`
	Username string            `json:"username"`
	Email    string            `json:"email"`
	Roles    []string          `json:"roles"`
	Scopes   []string          `json:"scopes"`
	Metadata map[string]string `json:"metadata"`
}

// APIKey API密钥信息
type APIKey struct {
	ID          string            `json:"id"`
	Key         string            `json:"key"`
	Secret      string            `json:"secret"`
	UserID      string            `json:"user_id"`
	Name        string            `json:"name"`
	Scopes      []string          `json:"scopes"`
	RateLimit   int               `json:"rate_limit"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	ExpiresAt   *time.Time        `json:"expires_at"`
	LastUsedAt  *time.Time        `json:"last_used_at"`
	IsActive    bool              `json:"is_active"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	Strategy    AuthStrategy `json:"strategy" yaml:"strategy"`
	JWTSecret   string       `json:"jwt_secret" yaml:"jwt_secret"`
	TokenExpiry time.Duration `json:"token_expiry" yaml:"token_expiry"`
	Issuer      string       `json:"issuer" yaml:"issuer"`
	Audience    string       `json:"audience" yaml:"audience"`
}

// Authenticator 认证器
type Authenticator struct {
	config    *AuthConfig
	apiKeys   map[string]*APIKey
	users     map[string]*User
	logger    *zap.Logger
}

// NewAuthenticator 创建认证器
func NewAuthenticator(config *AuthConfig, logger *zap.Logger) *Authenticator {
	return &Authenticator{
		config:  config,
		apiKeys: make(map[string]*APIKey),
		users:   make(map[string]*User),
		logger:  logger,
	}
}

// Middleware 认证中间件
func (a *Authenticator) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch a.config.Strategy {
		case NoAuth:
			c.Next()
			return
		case APIKey:
			if err := a.validateAPIKey(c); err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "unauthorized",
					"message": err.Error(),
				})
				c.Abort()
				return
			}
		case JWT:
			if err := a.validateJWT(c); err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "unauthorized",
					"message": err.Error(),
				})
				c.Abort()
				return
			}
		case BasicAuth:
			if err := a.validateBasicAuth(c); err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "unauthorized",
					"message": err.Error(),
				})
				c.Abort()
				return
			}
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "invalid auth strategy",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// validateAPIKey 验证API密钥
func (a *Authenticator) validateAPIKey(c *gin.Context) error {
	// 从头部获取API密钥
	apiKey := c.GetHeader("X-API-Key")
	if apiKey == "" {
		apiKey = c.Query("api_key")
	}

	if apiKey == "" {
		return fmt.Errorf("API key required")
	}

	// 查找API密钥
	keyInfo, exists := a.apiKeys[apiKey]
	if !exists {
		a.logger.Warn("Invalid API key attempted", zap.String("key", apiKey[:8]+"..."))
		return fmt.Errorf("invalid API key")
	}

	// 检查密钥是否激活
	if !keyInfo.IsActive {
		return fmt.Errorf("API key is disabled")
	}

	// 检查过期时间
	if keyInfo.ExpiresAt != nil && keyInfo.ExpiresAt.Before(time.Now()) {
		return fmt.Errorf("API key expired")
	}

	// 更新最后使用时间
	now := time.Now()
	keyInfo.LastUsedAt = &now

	// 设置用户信息到上下文
	if user, exists := a.users[keyInfo.UserID]; exists {
		c.Set("user", user)
	}
	c.Set("api_key", keyInfo)

	a.logger.Info("API key validated",
		zap.String("key_id", keyInfo.ID),
		zap.String("user_id", keyInfo.UserID))

	return nil
}

// validateJWT 验证JWT令牌
func (a *Authenticator) validateJWT(c *gin.Context) error {
	// 从头部获取JWT令牌
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return fmt.Errorf("authorization header required")
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		return fmt.Errorf("bearer token required")
	}

	// 解析JWT令牌
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(a.config.JWTSecret), nil
	})

	if err != nil {
		a.logger.Warn("JWT validation failed", zap.Error(err))
		return fmt.Errorf("invalid token: %w", err)
	}

	if !token.Valid {
		return fmt.Errorf("invalid token")
	}

	// 提取用户信息
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		user := &User{
			ID:       getStringClaim(claims, "sub"),
			Username: getStringClaim(claims, "username"),
			Email:    getStringClaim(claims, "email"),
			Roles:    getStringSliceClaim(claims, "roles"),
			Scopes:   getStringSliceClaim(claims, "scopes"),
		}

		c.Set("user", user)
		c.Set("jwt_claims", claims)

		a.logger.Info("JWT validated",
			zap.String("user_id", user.ID),
			zap.String("username", user.Username))
	}

	return nil
}

// validateBasicAuth 验证基本认证
func (a *Authenticator) validateBasicAuth(c *gin.Context) error {
	username, password, ok := c.Request.BasicAuth()
	if !ok {
		return fmt.Errorf("basic auth credentials required")
	}

	// 查找用户
	var user *User
	for _, u := range a.users {
		if u.Username == username {
			user = u
			break
		}
	}

	if user == nil {
		a.logger.Warn("User not found for basic auth", zap.String("username", username))
		return fmt.Errorf("invalid credentials")
	}

	// 验证密码（这里简化处理，实际应该使用加密密码）
	expectedPassword := a.hashPassword(password)
	if user.Metadata["password_hash"] != expectedPassword {
		return fmt.Errorf("invalid credentials")
	}

	c.Set("user", user)

	a.logger.Info("Basic auth validated",
		zap.String("user_id", user.ID),
		zap.String("username", user.Username))

	return nil
}

// CreateAPIKey 创建API密钥
func (a *Authenticator) CreateAPIKey(userID, name string, scopes []string, rateLimit int, expiresAt *time.Time) (*APIKey, error) {
	// 生成API密钥
	key := a.generateAPIKey()
	secret := a.generateSecret()

	apiKey := &APIKey{
		ID:         fmt.Sprintf("ak_%d", time.Now().UnixNano()),
		Key:        key,
		Secret:     secret,
		UserID:     userID,
		Name:       name,
		Scopes:     scopes,
		RateLimit:  rateLimit,
		Metadata:   make(map[string]string),
		CreatedAt:  time.Now(),
		ExpiresAt:  expiresAt,
		IsActive:   true,
	}

	a.apiKeys[key] = apiKey

	a.logger.Info("API key created",
		zap.String("key_id", apiKey.ID),
		zap.String("user_id", userID),
		zap.String("name", name))

	return apiKey, nil
}

// RevokeAPIKey 撤销API密钥
func (a *Authenticator) RevokeAPIKey(key string) error {
	if apiKey, exists := a.apiKeys[key]; exists {
		apiKey.IsActive = false
		a.logger.Info("API key revoked", zap.String("key_id", apiKey.ID))
		return nil
	}
	return fmt.Errorf("API key not found")
}

// CreateJWT 创建JWT令牌
func (a *Authenticator) CreateJWT(user *User) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":       user.ID,
		"username":  user.Username,
		"email":     user.Email,
		"roles":     user.Roles,
		"scopes":    user.Scopes,
		"iat":       now.Unix(),
		"exp":       now.Add(a.config.TokenExpiry).Unix(),
		"iss":       a.config.Issuer,
		"aud":       a.config.Audience,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(a.config.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("failed to create token: %w", err)
	}

	a.logger.Info("JWT created",
		zap.String("user_id", user.ID),
		zap.String("username", user.Username))

	return tokenString, nil
}

// AddUser 添加用户
func (a *Authenticator) AddUser(user *User) {
	a.users[user.ID] = user
	a.logger.Info("User added",
		zap.String("user_id", user.ID),
		zap.String("username", user.Username))
}

// GetUser 获取用户
func (a *Authenticator) GetUser(userID string) (*User, bool) {
	user, exists := a.users[userID]
	return user, exists
}

// ListAPIKeys 列出用户的API密钥
func (a *Authenticator) ListAPIKeys(userID string) []*APIKey {
	keys := make([]*APIKey, 0)
	for _, key := range a.apiKeys {
		if key.UserID == userID {
			keys = append(keys, key)
		}
	}
	return keys
}

// GetCurrentUser 从上下文获取当前用户
func GetCurrentUser(c *gin.Context) (*User, bool) {
	if user, exists := c.Get("user"); exists {
		return user.(*User), true
	}
	return nil, false
}

// HasScope 检查用户是否有指定权限
func HasScope(c *gin.Context, scope string) bool {
	if user, exists := GetCurrentUser(c); exists {
		for _, s := range user.Scopes {
			if s == scope || s == "*" {
				return true
			}
		}
	}

	if apiKey, exists := c.Get("api_key"); exists {
		key := apiKey.(*APIKey)
		for _, s := range key.Scopes {
			if s == scope || s == "*" {
				return true
			}
		}
	}

	return false
}

// RequireScope 权限检查中间件
func RequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !HasScope(c, scope) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "insufficient permissions",
				"required_scope": scope,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// 辅助函数
func getStringClaim(claims jwt.MapClaims, key string) string {
	if val, ok := claims[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getStringSliceClaim(claims jwt.MapClaims, key string) []string {
	if val, ok := claims[key]; ok {
		if slice, ok := val.([]interface{}); ok {
			result := make([]string, len(slice))
			for i, v := range slice {
				if str, ok := v.(string); ok {
					result[i] = str
				}
			}
			return result
		}
	}
	return []string{}
}

func (a *Authenticator) generateAPIKey() string {
	// 生成32字符的API密钥
	return fmt.Sprintf("envdata_%s", a.generateRandomString(24))
}

func (a *Authenticator) generateSecret() string {
	// 生成64字符的密钥
	return a.generateRandomString(64)
}

func (a *Authenticator) generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(result)
}

func (a *Authenticator) hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}