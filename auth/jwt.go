package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrMissingSecret = errors.New("jwt secret not configured")
	ErrInvalidToken  = errors.New("invalid token")
)

const (
	// issuerValue 为通用签发者标识，适用于所有依赖该公共包的服务。
	issuerValue = "github.com/yinqf/go-pkg"
)

type contextKey string

// claimsContextKey 作为 context.WithValue 的 key，保证读取 claims 时不会与其他 key 冲突。
const claimsContextKey contextKey = "github.com/yinqf/go-pkg/auth/claims"

// Claims 封装 jwt.RegisteredClaims，便于多服务共享鉴权信息。
type Claims struct {
	jwt.RegisteredClaims
}

var (
	secretOnce sync.Once
	secret     []byte
	secretErr  error
)

// GenerateToken 根据 subject 与有效期生成签名后的 JWT。
func GenerateToken(subject string, ttl time.Duration) (string, error) {
	if subject == "" {
		return "", errors.New("subject is required")
	}

	secretValue, err := getSecret()
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			Issuer:    issuerValue,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	if ttl > 0 {
		claims.ExpiresAt = jwt.NewNumericDate(now.Add(ttl))
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secretValue)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return signed, nil
}

// ParseToken 校验签名并返回解析出的 claims。
func ParseToken(token string) (*Claims, error) {
	if token == "" {
		return nil, fmt.Errorf("%w: empty token", ErrInvalidToken)
	}

	secretValue, err := getSecret()
	if err != nil {
		return nil, err
	}

	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %s", t.Method.Alg())
		}
		return secretValue, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	if !parsed.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// ContextWithClaims 将 claims 存入上下文，方便后续链路读取。
func ContextWithClaims(ctx context.Context, claims *Claims) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, claimsContextKey, claims)
}

// ClaimsFromContext 从上下文中提取 claims，若不存在返回 false。
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	if ctx == nil {
		return nil, false
	}
	claims, ok := ctx.Value(claimsContextKey).(*Claims)
	return claims, ok
}

func getSecret() ([]byte, error) {
	secretOnce.Do(func() {
		value := os.Getenv("JWT_SECRET")
		if value == "" {
			secretErr = ErrMissingSecret
			return
		}
		secret = []byte(value)
	})

	return secret, secretErr
}

// ResetCacheForTest 清理缓存的密钥，便于测试重新配置环境变量。
func ResetCacheForTest() {
	secretOnce = sync.Once{}
	secret = nil
	secretErr = nil
}
