// Package auth provides JWT authentication and RBAC middleware for the MSE API.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Role represents a user role with specific permissions.
type Role string

const (
	RoleAdmin   Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer  Role = "viewer"
)

// Permission represents a specific action that can be performed.
type Permission string

const (
	PermissionReadMigrations   Permission = "read:migrations"
	PermissionWriteMigrations  Permission = "write:migrations"
	PermissionDeleteMigrations Permission = "delete:migrations"
	PermissionReadSafety       Permission = "read:safety"
	PermissionManageUsers      Permission = "manage:users"
	PermissionManageSettings   Permission = "manage:settings"
)

// rolePermissions maps roles to their allowed permissions.
var rolePermissions = map[Role][]Permission{
	RoleAdmin: {
		PermissionReadMigrations,
		PermissionWriteMigrations,
		PermissionDeleteMigrations,
		PermissionReadSafety,
		PermissionManageUsers,
		PermissionManageSettings,
	},
	RoleOperator: {
		PermissionReadMigrations,
		PermissionWriteMigrations,
		PermissionReadSafety,
	},
	RoleViewer: {
		PermissionReadMigrations,
		PermissionReadSafety,
	},
}

// Claims represents the JWT claims.
type Claims struct {
	Username string `json:"username"`
	Role     Role   `json:"role"`
	jwt.RegisteredClaims
}

// User represents a authenticated user.
type User struct {
	Username string
	Role     Role
}

// contextKey is the type for context keys in this package.
type contextKey string

const userContextKey contextKey = "user"

// Auth handles JWT token generation and validation.
type Auth struct {
	secret     []byte
	expiry     time.Duration
	issuer     string
	users      map[string]UserRecord
}

// UserRecord stores user credentials (in production, use a database).
type UserRecord struct {
	Username     string
	PasswordHash string // In production, use bcrypt
	Role         Role
}

// NewAuth creates a new Auth instance.
func NewAuth(secret string, expiry time.Duration) *Auth {
	if secret == "" {
		// Generate a random secret if none provided
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			panic("failed to generate random secret: " + err.Error())
		}
		secret = hex.EncodeToString(b)
	}

	a := &Auth{
		secret: []byte(secret),
		expiry: expiry,
		issuer: "mse-engine",
		users:  make(map[string]UserRecord),
	}

	// Seed default admin user (in production, use a proper user store)
	a.users["admin"] = UserRecord{
		Username:     "admin",
		PasswordHash: "admin123", // In production, use bcrypt
		Role:         RoleAdmin,
	}
	a.users["operator"] = UserRecord{
		Username:     "operator",
		PasswordHash: "operator123",
		Role:         RoleOperator,
	}
	a.users["viewer"] = UserRecord{
		Username:     "viewer",
		PasswordHash: "viewer123",
		Role:         RoleViewer,
	}

	return a
}

// GenerateToken creates a new JWT token for a user.
func (a *Auth) GenerateToken(username string, role Role) (string, error) {
	now := time.Now()
	claims := &Claims{
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    a.issuer,
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(a.expiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.secret)
}

// ValidateToken validates a JWT token and returns the claims.
func (a *Auth) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.secret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// Authenticate validates username/password and returns a token.
func (a *Auth) Authenticate(username, password string) (string, Role, error) {
	user, exists := a.users[username]
	if !exists {
		return "", "", errors.New("invalid credentials")
	}

	// In production, use bcrypt.CompareHashAndPassword
	if user.PasswordHash != password {
		return "", "", errors.New("invalid credentials")
	}

	token, err := a.GenerateToken(username, user.Role)
	if err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}

	return token, user.Role, nil
}

// UserFromContext extracts the user from the context.
func UserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(userContextKey).(*User)
	return user, ok
}

// ContextWithUser adds a user to the context.
func ContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// RequireAuth is middleware that requires a valid JWT token.
func (a *Auth) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, "authorization header format must be Bearer {token}", http.StatusUnauthorized)
			return
		}

		claims, err := a.ValidateToken(parts[1])
		if err != nil {
			http.Error(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		user := &User{
			Username: claims.Username,
			Role:     claims.Role,
		}

		ctx := ContextWithUser(r.Context(), user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequirePermission is middleware that requires a specific permission.
func (a *Auth) RequirePermission(permission Permission, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if !a.HasPermission(user.Role, permission) {
			http.Error(w, "forbidden: insufficient permissions", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// HasPermission checks if a role has a specific permission.
func (a *Auth) HasPermission(role Role, permission Permission) bool {
	permissions, exists := rolePermissions[role]
	if !exists {
		return false
	}

	for _, p := range permissions {
		if p == permission {
			return true
		}
	}

	return false
}
