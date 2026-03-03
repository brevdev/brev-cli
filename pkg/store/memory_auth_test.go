package store

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

func TestMemoryAuthStore_SaveAndGet_RoundTrips(t *testing.T) {
	s := NewMemoryAuthStore()
	tokens := entity.AuthTokens{AccessToken: "access", RefreshToken: "refresh"}

	if err := s.SaveAuthTokens(tokens); err != nil {
		t.Fatalf("SaveAuthTokens: %v", err)
	}

	got, err := s.GetAuthTokens()
	if err != nil {
		t.Fatalf("GetAuthTokens: %v", err)
	}
	if got.AccessToken != "access" || got.RefreshToken != "refresh" {
		t.Fatalf("unexpected tokens: %+v", got)
	}
}

func TestMemoryAuthStore_GetAuthTokens_WhenEmpty_ReturnsCredentialsFileNotFound(t *testing.T) {
	s := NewMemoryAuthStore()

	_, err := s.GetAuthTokens()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if _, ok := err.(*breverrors.CredentialsFileNotFound); !ok {
		t.Fatalf("expected *CredentialsFileNotFound, got %T: %v", err, err)
	}
}

func TestMemoryAuthStore_SaveAuthTokens_EmptyAccessToken_ReturnsError(t *testing.T) {
	s := NewMemoryAuthStore()

	err := s.SaveAuthTokens(entity.AuthTokens{AccessToken: "", RefreshToken: "refresh"})
	if err == nil {
		t.Fatal("expected error for empty access token, got nil")
	}
}

func TestMemoryAuthStore_DeleteAuthTokens_ClearsTokens(t *testing.T) {
	s := NewMemoryAuthStore()
	_ = s.SaveAuthTokens(entity.AuthTokens{AccessToken: "access", RefreshToken: "refresh"})

	if err := s.DeleteAuthTokens(); err != nil {
		t.Fatalf("DeleteAuthTokens: %v", err)
	}

	_, err := s.GetAuthTokens()
	if _, ok := err.(*breverrors.CredentialsFileNotFound); !ok {
		t.Fatalf("expected *CredentialsFileNotFound after delete, got %T: %v", err, err)
	}
}

func TestMemoryAuthStore_Seed_PreFillsTokens(t *testing.T) {
	s := NewMemoryAuthStore()
	s.Seed(entity.AuthTokens{AccessToken: "seeded-access", RefreshToken: "seeded-refresh"})

	got, err := s.GetAuthTokens()
	if err != nil {
		t.Fatalf("GetAuthTokens after Seed: %v", err)
	}
	if got.AccessToken != "seeded-access" || got.RefreshToken != "seeded-refresh" {
		t.Fatalf("unexpected tokens after Seed: %+v", got)
	}
}
