// Package client - auth.go
//
// 第二刀 P1.2 — JWT bearer auth(per D66=B 4-claim,2026-08-20)。
//
// 设计原则:
//   - token 从 ~/.wau/credentials 读(per D74)
//   - 格式:{"access_token": "...", "refresh_token": "...", "expires_at": ..., "user_id": "..."}
//   - 发送 Authorization: Bearer <access_token>
//   - 401 自动 refresh(用 refresh_token),再 retry 1 次
//   - 向后兼容:仍发 X-Agent-Role(老 server 不读 Bearer)
package client

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Credentials wau-cli 用户凭证(对应 wau-agent login 的输出)。
type Credentials struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    int64  `json:"expires_at,omitempty"` // unix seconds
	UserID       string `json:"user_id,omitempty"`
	Endpoint     string `json:"endpoint,omitempty"`
}

// DefaultCredentialsPath 返回默认凭证文件路径。
func DefaultCredentialsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".wau", "credentials")
}

// LoadCredentials 从 path 读凭证。文件不存在返回空凭证(非错误)。
func LoadCredentials(path string) (*Credentials, error) {
	if path == "" {
		path = DefaultCredentialsPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Credentials{}, nil
		}
		return nil, fmt.Errorf("read credentials %s: %w", path, err)
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	return &creds, nil
}

// Save 写凭证到 path(JSON,0600 权限)。
func (c *Credentials) Save(path string) error {
	if path == "" {
		path = DefaultCredentialsPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("mkdir credentials dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// Valid 检查 access_token 是否未过期。
func (c *Credentials) Valid() bool {
	if c == nil || c.AccessToken == "" {
		return false
	}
	if c.ExpiresAt == 0 {
		// 没有过期时间,认为永久有效(测试用)
		return true
	}
	return time.Now().Unix() < c.ExpiresAt
}

// AuthProvider 提供 client 用的 token。
//
// 实现:
//   - StaticToken:写死一个 token(测试用)
//   - CredentialsProvider:从 Credentials 读 + 缓存
//   - RefreshableProvider:401 时自动 refresh
type AuthProvider interface {
	// Token 返回当前可用 token;若返回 "",header 不加 Authorization。
	Token(ctx interface{}) (string, error)
	// Refresh 用 refresh_token 换新 token(可选,实现可返回 ErrNoRefresh)。
	Refresh(ctx interface{}) error
}

// CredentialsProvider 简单凭证 provider(不自动 refresh)。
type CredentialsProvider struct {
	creds *Credentials
	mu    sync.RWMutex
}

// NewCredentialsProvider 构造。
func NewCredentialsProvider(creds *Credentials) *CredentialsProvider {
	return &CredentialsProvider{creds: creds}
}

// Token 返回 access_token。
func (p *CredentialsProvider) Token(_ interface{}) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.creds == nil || p.creds.AccessToken == "" {
		return "", nil
	}
	if !p.creds.Valid() {
		return "", fmt.Errorf("access token expired at %s", time.Unix(p.creds.ExpiresAt, 0).Format(time.RFC3339))
	}
	return p.creds.AccessToken, nil
}

// Refresh 默认 no-op。
func (p *CredentialsProvider) Refresh(_ interface{}) error {
	return fmt.Errorf("no refresh configured")
}

// Set 更新凭证(用于 login 后)。
func (p *CredentialsProvider) Set(c *Credentials) {
	p.mu.Lock()
	p.creds = c
	p.mu.Unlock()
}
