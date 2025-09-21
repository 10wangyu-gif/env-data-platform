package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/env-data-platform/internal/config"
)

// Claims JWT声明
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	RoleID   uint   `json:"role_id"`
	RoleName string `json:"role_name"`
	jwt.RegisteredClaims
}

// JWTManager JWT管理器
type JWTManager struct {
	secretKey string
	issuer    string
	expire    time.Duration
}

// NewJWTManager 创建JWT管理器
func NewJWTManager(cfg *config.Config) *JWTManager {
	return &JWTManager{
		secretKey: cfg.JWT.Secret,
		issuer:    cfg.JWT.Issuer,
		expire:    cfg.JWT.Expire,
	}
}

// GenerateToken 生成JWT令牌
func (j *JWTManager) GenerateToken(userID uint, username string, roleID uint, roleName string) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:   userID,
		Username: username,
		RoleID:   roleID,
		RoleName: roleName,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.expire)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.secretKey))
}

// ParseToken 解析JWT令牌
func (j *JWTManager) ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(j.secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// RefreshToken 刷新令牌
func (j *JWTManager) RefreshToken(tokenString string) (string, error) {
	claims, err := j.ParseToken(tokenString)
	if err != nil {
		return "", err
	}

	// 检查令牌是否在30分钟内过期
	if time.Until(claims.ExpiresAt.Time) > 30*time.Minute {
		return "", errors.New("token is not eligible for refresh")
	}

	return j.GenerateToken(claims.UserID, claims.Username, claims.RoleID, claims.RoleName)
}

// ValidateToken 验证令牌有效性
func (j *JWTManager) ValidateToken(tokenString string) bool {
	_, err := j.ParseToken(tokenString)
	return err == nil
}