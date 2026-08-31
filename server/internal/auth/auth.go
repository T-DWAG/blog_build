package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// 哨兵错误
var ErrNoSecret = errors.New("auth: no secret")

const (
	tokenTTL    = 12 * time.Hour
	bcryptCost  = 10
	tokenIssuer = "blog"
)

// Hash 对明文密码做 bcrypt 哈希（cost 10）。
func Hash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Compare 校验明文密码与 bcrypt 哈希是否匹配。
func Compare(password, passwordHash string) error {
	return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
}

// claims 是签发的 JWT 载荷，仅携带站主用户名。
type claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Issue 以 secret 签发 12h 有效的 JWT。secret 为空拒绝签发。
func Issue(username, secret string) (string, error) {
	if secret == "" {
		return "", ErrNoSecret
	}
	now := time.Now()
	c := claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tokenIssuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(secret))
}

// Parse 校验并解析 JWT，返回站主用户名。签名不符/过期/格式错都返回 error。
func Parse(tokenString, secret string) (string, error) {
	if secret == "" {
		return "", ErrNoSecret
	}
	token, err := jwt.ParseWithClaims(tokenString, &claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return "", err
	}
	c, ok := token.Claims.(*claims)
	if !ok || !token.Valid {
		return "", errors.New("invalid token")
	}
	return c.Username, nil
}
