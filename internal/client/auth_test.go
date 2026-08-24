package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─── Login (P4.3) ──────────────────────────────────────────────────────────

func TestLogin_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/l5/login" {
			t.Errorf("path = %q, want /v1/l5/login", r.URL.Path)
		}
		var req L5LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode req: %v", err)
		}
		if req.Username == "" || req.Password == "" {
			t.Error("username/password missing")
		}
		_ = json.NewEncoder(w).Encode(L5LoginResponse{
			OK:           true,
			AccessToken:  "test-access-token",
			RefreshToken: "test-refresh-token",
			ExpiresAt:    time.Now().Add(1 * time.Hour).Unix(),
			UserID:       "test-user-id",
		})
	}))
	defer srv.Close()

	creds, err := Login(context.Background(), LoginOptions{
		BaseURL:  srv.URL,
		Role:     "external_agent",
		Username: "alice",
		Password: "secret",
		Endpoint: "http://example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if creds.AccessToken != "test-access-token" {
		t.Errorf("AccessToken = %q", creds.AccessToken)
	}
	if creds.RefreshToken != "test-refresh-token" {
		t.Errorf("RefreshToken = %q", creds.RefreshToken)
	}
	if creds.UserID != "test-user-id" {
		t.Errorf("UserID = %q", creds.UserID)
	}
	if creds.Endpoint != "http://example.com" {
		t.Errorf("Endpoint = %q", creds.Endpoint)
	}
	if creds.ExpiresAt == 0 {
		t.Error("ExpiresAt should be set")
	}
}

func TestLogin_BadCreds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(L5LoginResponse{
			OK:    false,
			Error: "invalid credentials",
		})
	}))
	defer srv.Close()

	_, err := Login(context.Background(), LoginOptions{
		BaseURL:  srv.URL,
		Username: "alice",
		Password: "wrong",
	})
	if err == nil {
		t.Fatal("want error for bad creds")
	}
	if !strings.Contains(err.Error(), "invalid credentials") {
		t.Errorf("err = %v, want 'invalid credentials'", err)
	}
}

func TestLogin_MissingFields(t *testing.T) {
	_, err := Login(context.Background(), LoginOptions{
		Username: "alice",
		// Password missing
	})
	if err == nil {
		t.Fatal("want error for missing password")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("err = %v", err)
	}
}

func TestLogin_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := Login(context.Background(), LoginOptions{
		BaseURL:  srv.URL,
		Username: "alice",
		Password: "secret",
	})
	if err == nil {
		t.Fatal("want error for 500")
	}
}

// ─── Credentials Save / Load roundtrip ─────────────────────────────────────

func TestSaveLoadCredentials_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	original := &Credentials{
		AccessToken:  "access-1",
		RefreshToken: "refresh-1",
		ExpiresAt:    1234567890,
		UserID:       "user-1",
		Endpoint:     "http://example.com",
	}
	if err := original.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := LoadCredentials(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != original.AccessToken ||
		got.RefreshToken != original.RefreshToken ||
		got.ExpiresAt != original.ExpiresAt ||
		got.UserID != original.UserID ||
		got.Endpoint != original.Endpoint {
		t.Errorf("roundtrip mismatch:\n  got %+v\n  want %+v", got, original)
	}
}

func TestSaveCredentials_FilePermission(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	c := &Credentials{AccessToken: "x"}
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	mode := info.Mode().Perm()
	if mode != 0o600 {
		t.Errorf("file mode = %o, want 0600", mode)
	}
}

func TestLoadCredentials_NotExist(t *testing.T) {
	c, err := LoadCredentials("/nonexistent/path/creds.json")
	if err != nil {
		t.Fatalf("want nil err for missing file, got: %v", err)
	}
	if c.AccessToken != "" || c.UserID != "" {
		t.Errorf("want empty Credentials, got %+v", c)
	}
}

func TestLoadCredentials_DefaultPath(t *testing.T) {
	// 调用 "" 应该用默认 ~/.wau/credentials(不存在的 → 空 creds)
	c, err := LoadCredentials("")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if c == nil {
		t.Fatal("want non-nil empty Credentials")
	}
}

func TestDefaultCredentialsPath(t *testing.T) {
	got := DefaultCredentialsPath()
	if !strings.HasSuffix(got, ".wau/credentials") {
		t.Errorf("DefaultCredentialsPath = %q, want suffix .wau/credentials", got)
	}
}

// ─── Valid ────────────────────────────────────────────────────────────────

func TestValid_FreshToken(t *testing.T) {
	c := &Credentials{
		AccessToken: "x",
		ExpiresAt:   time.Now().Add(1 * time.Hour).Unix(),
	}
	if !c.Valid() {
		t.Error("fresh token should be valid")
	}
}

func TestValid_ExpiredToken(t *testing.T) {
	c := &Credentials{
		AccessToken: "x",
		ExpiresAt:   time.Now().Add(-1 * time.Hour).Unix(),
	}
	if c.Valid() {
		t.Error("expired token should NOT be valid")
	}
}

func TestValid_NoExpiry(t *testing.T) {
	c := &Credentials{AccessToken: "x", ExpiresAt: 0}
	if !c.Valid() {
		t.Error("token without expiry should be valid (permanent)")
	}
}

func TestValid_NilOrEmpty(t *testing.T) {
	var nilc *Credentials
	if nilc.Valid() {
		t.Error("nil Credentials should NOT be valid")
	}
	if (&Credentials{}).Valid() {
		t.Error("empty Credentials should NOT be valid")
	}
}

// ─── CredentialsProvider ───────────────────────────────────────────────────

func TestCredentialsProvider_Token(t *testing.T) {
	c := &Credentials{AccessToken: "x", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	p := NewCredentialsProvider(c)
	tok, err := p.Token(nil)
	if err != nil {
		t.Fatal(err)
	}
	if tok != "x" {
		t.Errorf("Token = %q", tok)
	}
}

func TestCredentialsProvider_Expired(t *testing.T) {
	c := &Credentials{AccessToken: "x", ExpiresAt: time.Now().Add(-time.Hour).Unix()}
	p := NewCredentialsProvider(c)
	_, err := p.Token(nil)
	if err == nil {
		t.Fatal("want error for expired token")
	}
	if !errors.Is(err, err) || !strings.Contains(err.Error(), "expired") {
		t.Logf("err = %v", err)
	}
}