package auth

import (
	"testing"
	"time"
)

func TestGenerateToken(t *testing.T) {
	a := NewAuth("test-secret", time.Hour)

	token, err := a.GenerateToken("admin", RoleAdmin)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	if token == "" {
		t.Fatal("GenerateToken() returned empty token")
	}
}

func TestValidateToken(t *testing.T) {
	a := NewAuth("test-secret", time.Hour)

	token, err := a.GenerateToken("admin", RoleAdmin)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	claims, err := a.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}

	if claims.Username != "admin" {
		t.Errorf("ValidateToken() username = %v, want %v", claims.Username, "admin")
	}

	if claims.Role != RoleAdmin {
		t.Errorf("ValidateToken() role = %v, want %v", claims.Role, RoleAdmin)
	}
}

func TestValidateTokenInvalid(t *testing.T) {
	a := NewAuth("test-secret", time.Hour)

	_, err := a.ValidateToken("invalid-token")
	if err == nil {
		t.Fatal("ValidateToken() expected error for invalid token")
	}
}

func TestValidateTokenWrongSecret(t *testing.T) {
	a1 := NewAuth("secret1", time.Hour)
	a2 := NewAuth("secret2", time.Hour)

	token, err := a1.GenerateToken("admin", RoleAdmin)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	_, err = a2.ValidateToken(token)
	if err == nil {
		t.Fatal("ValidateToken() expected error for wrong secret")
	}
}

func TestAuthenticate(t *testing.T) {
	a := NewAuth("test-secret", time.Hour)

	token, role, err := a.Authenticate("admin", "admin123")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	if token == "" {
		t.Fatal("Authenticate() returned empty token")
	}

	if role != RoleAdmin {
		t.Errorf("Authenticate() role = %v, want %v", role, RoleAdmin)
	}
}

func TestAuthenticateInvalidCredentials(t *testing.T) {
	a := NewAuth("test-secret", time.Hour)

	_, _, err := a.Authenticate("admin", "wrong-password")
	if err == nil {
		t.Fatal("Authenticate() expected error for invalid credentials")
	}
}

func TestHasPermission(t *testing.T) {
	a := NewAuth("test-secret", time.Hour)

	tests := []struct {
		role       Role
		permission Permission
		want       bool
	}{
		{RoleAdmin, PermissionReadMigrations, true},
		{RoleAdmin, PermissionWriteMigrations, true},
		{RoleAdmin, PermissionDeleteMigrations, true},
		{RoleAdmin, PermissionManageUsers, true},
		{RoleOperator, PermissionReadMigrations, true},
		{RoleOperator, PermissionWriteMigrations, true},
		{RoleOperator, PermissionDeleteMigrations, false},
		{RoleViewer, PermissionReadMigrations, true},
		{RoleViewer, PermissionWriteMigrations, false},
		{RoleViewer, PermissionDeleteMigrations, false},
	}

	for _, tt := range tests {
		got := a.HasPermission(tt.role, tt.permission)
		if got != tt.want {
			t.Errorf("HasPermission(%v, %v) = %v, want %v", tt.role, tt.permission, got, tt.want)
		}
	}
}
