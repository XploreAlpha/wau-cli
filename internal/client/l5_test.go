// Package client - l5_test.go
//
// 第二刀 P1.3 — L5 包管理器 JSON roundtrip 测试。
//
// 覆盖:L5Search / L5Update / L5Login JSON roundtrip。
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestL5Search_Roundtrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/l5/search" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var req L5SearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
		}
		if req.Query != "medical" {
			t.Errorf("Query = %q", req.Query)
		}
		w.WriteHeader(200)
		fmt.Fprintln(w, `{"ok":true,"results":[{"agent_id":"a-1","name":"fox-medical","version":"1.2.0","trust_score":0.92,"author":"acme"}],"total":1}`)
	}))
	defer srv.Close()

	c := NewClient(Options{BaseURL: srv.URL})
	resp, err := c.L5Search(context.Background(), &L5SearchRequest{Query: "medical", Universe: "medical"})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK || resp.Total != 1 || len(resp.Results) != 1 {
		t.Fatalf("resp = %+v", resp)
	}
	if resp.Results[0].Name != "fox-medical" || resp.Results[0].TrustScore != 0.92 {
		t.Errorf("results[0] = %+v", resp.Results[0])
	}
}

func TestL5Update_Roundtrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, `{"ok":true,"updated_count":2,"updated_agents":["fox-medical","chinese-medicine"]}`)
	}))
	defer srv.Close()

	c := NewClient(Options{BaseURL: srv.URL})
	resp, err := c.L5Update(context.Background(), &L5UpdateRequest{UserID: "u1", AgentName: "fox-medical"})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK || resp.UpdatedCount != 2 || len(resp.UpdatedAgents) != 2 {
		t.Errorf("resp = %+v", resp)
	}
}

func TestL5Login_Roundtrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, `{"ok":true,"access_token":"new-tok","refresh_token":"ref","expires_at":1700003600,"user_id":"u1"}`)
	}))
	defer srv.Close()

	c := NewClient(Options{BaseURL: srv.URL})
	resp, err := c.L5Login(context.Background(), &L5LoginRequest{Username: "alice", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK || resp.AccessToken != "new-tok" || resp.ExpiresAt == 0 {
		t.Errorf("resp = %+v", resp)
	}
}

func TestCredentials_LoadSaveRoundtrip(t *testing.T) {
	tmp := t.TempDir() + "/creds.json"
	want := &Credentials{AccessToken: "tok", RefreshToken: "ref", ExpiresAt: 1700003600, UserID: "u1"}
	if err := want.Save(tmp); err != nil {
		t.Fatal(err)
	}
	got, err := LoadCredentials(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken || got.ExpiresAt != want.ExpiresAt || got.UserID != want.UserID {
		t.Errorf("got = %+v, want = %+v", got, want)
	}
}

func TestCredentials_Valid(t *testing.T) {
	now := time.Now().Unix()
	cases := []struct {
		name string
		c    *Credentials
		want bool
	}{
		{"nil", nil, false},
		{"empty token", &Credentials{}, false},
		{"no expiry", &Credentials{AccessToken: "x"}, true},
		{"future expiry", &Credentials{AccessToken: "x", ExpiresAt: now + 3600}, true},
		{"past expiry", &Credentials{AccessToken: "x", ExpiresAt: now - 3600}, false},
	}
	for _, tc := range cases {
		if got := tc.c.Valid(); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}