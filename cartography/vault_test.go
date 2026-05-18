package cartography

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSplitMountPath(t *testing.T) {
	cases := []struct {
		in            string
		mount, key    string
	}{
		{"secret/go-trader/fred", "secret", "go-trader/fred"},
		{"/secret/go-trader/fred/", "secret", "go-trader/fred"},
		{"kv/foo", "kv", "foo"},
		{"oneSegment", "", ""},
		{"", "", ""},
	}
	for _, c := range cases {
		m, k := splitMountPath(c.in)
		if m != c.mount || k != c.key {
			t.Errorf("splitMountPath(%q) = (%q,%q), want (%q,%q)",
				c.in, m, k, c.mount, c.key)
		}
	}
}

// TestVaultLoaderField uses an httptest server to verify the request shape
// (token header, /v1/<mount>/data/<key> path) and the KV-v2 response decode.
func TestVaultLoaderField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Vault-Token"); got != "test-token" {
			t.Errorf("missing/wrong vault token: %q", got)
		}
		if r.URL.Path != "/v1/secret/data/go-trader/fred" {
			t.Errorf("wrong path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"data":     map[string]interface{}{"api_key": "abc123"},
				"metadata": map[string]interface{}{"version": 1},
			},
		})
	}))
	defer srv.Close()

	v := &VaultLoader{Addr: srv.URL, Token: "test-token", HTTP: srv.Client()}
	got, err := v.Field(context.Background(), "secret/go-trader/fred", "api_key")
	if err != nil {
		t.Fatalf("Field: %v", err)
	}
	if got != "abc123" {
		t.Errorf("got %q, want abc123", got)
	}
}

func TestVaultLoaderMissingField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"data": map[string]interface{}{"other_field": "x"},
			},
		})
	}))
	defer srv.Close()
	v := &VaultLoader{Addr: srv.URL, Token: "tok", HTTP: srv.Client()}
	if _, err := v.Field(context.Background(), "secret/x/y", "api_key"); err == nil {
		t.Error("expected missing-field error, got nil")
	}
}

func TestVaultLoaderForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	v := &VaultLoader{Addr: srv.URL, Token: "tok", HTTP: srv.Client()}
	_, err := v.Field(context.Background(), "secret/x/y", "api_key")
	if err == nil {
		t.Fatal("expected forbidden error")
	}
}

func TestVaultLoaderNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	v := &VaultLoader{Addr: srv.URL, Token: "tok", HTTP: srv.Client()}
	_, err := v.Field(context.Background(), "secret/x/y", "api_key")
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestVaultLoaderInvalidPath(t *testing.T) {
	v := &VaultLoader{Addr: "http://x", Token: "tok"}
	if _, err := v.Field(context.Background(), "no-slash", "api_key"); err == nil {
		t.Error("expected invalid-path error")
	}
}
