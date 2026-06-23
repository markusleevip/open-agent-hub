package auth

import (
	"errors"
	"regexp"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/openagenthub/backend/internal/config"
	"golang.org/x/crypto/bcrypt"
)

// Claims represents JWT claims
type Claims struct {
	UserID      string   `json:"user_id"`
	Username    string   `json:"username"`
	WorkspaceID string   `json:"workspace_id"`
	OrgID       string   `json:"org_id"`
	Role        string   `json:"role"`
	Scopes      []string `json:"scopes"`
	jwt.RegisteredClaims
}

// GenerateJWT generates a JWT token
func GenerateJWT(cfg *config.Config, userID, username, orgID, workspaceID, role string, scopes []string) (string, error) {
	now := time.Now()
	expireAt := now.Add(time.Duration(cfg.JWTExpire) * time.Hour)

	claims := Claims{
		UserID:      userID,
		Username:    username,
		OrgID:       orgID,
		WorkspaceID: workspaceID,
		Role:        role,
		Scopes:      scopes,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expireAt),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "open-agent-hub",
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWTSecret))
}

// ParseJWT parses a JWT token
func ParseJWT(cfg *config.Config, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(cfg.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

// usernameRegexp defines username rules: letters/digits/CJK, must not start with a digit, 2-32 chars
var usernameRegexp = regexp.MustCompile(`^[a-zA-Z\x{4e00}-\x{9fff}][a-zA-Z0-9\x{4e00}-\x{9fff}]{1,31}$`)

// ValidateUsername validates the username format
func ValidateUsername(username string) error {
	if !usernameRegexp.MatchString(username) {
		return errors.New("username must be 2-32 chars, start with a letter or Chinese character, and contain only letters, digits, and Chinese characters")
	}
	return nil
}

// HashPassword hashes a password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash verifies a password hash
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
