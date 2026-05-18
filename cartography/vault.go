package cartography

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// Vault KV-v2 loader. Stdlib-only — no Vault SDK dependency.
//
// Reads VAULT_ADDR and VAULT_TOKEN from the environment (the standard
// Vault env vars; the operator already has these set if they use the CLI).
// Path syntax matches `vault kv` — e.g. "secret/go-trader/fred" — and is
// translated to the KV-v2 API endpoint /v1/<mount>/data/<key>.

// VaultLoader reads single fields from KV-v2 secrets.
type VaultLoader struct {
	Addr  string
	Token string
	HTTP  *http.Client
}

// NewVaultLoaderFromEnv builds a loader if VAULT_ADDR and VAULT_TOKEN are
// both set, returns nil + nil otherwise (Vault is optional infra).
func NewVaultLoaderFromEnv() *VaultLoader {
	addr := strings.TrimRight(os.Getenv("VAULT_ADDR"), "/")
	token := os.Getenv("VAULT_TOKEN")
	if addr == "" || token == "" {
		return nil
	}
	return &VaultLoader{
		Addr:  addr,
		Token: token,
		HTTP:  &http.Client{Timeout: 8 * time.Second},
	}
}

// Field reads a single named field from the secret at path. Path is the
// logical KV path (mount/key/...); the loader translates to /data/.
func (v *VaultLoader) Field(ctx context.Context, path, field string) (string, error) {
	if v == nil {
		return "", fmt.Errorf("vault not configured (VAULT_ADDR/VAULT_TOKEN)")
	}
	mount, key := splitMountPath(path)
	if mount == "" || key == "" {
		return "", fmt.Errorf("invalid vault path %q (need mount/key)", path)
	}
	url := fmt.Sprintf("%s/v1/%s/data/%s", v.Addr, mount, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Vault-Token", v.Token)
	resp, err := v.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("vault GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("vault: secret not found at %s", path)
	}
	if resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("vault: forbidden at %s — token lacks read on this path", path)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vault %s: HTTP %d", path, resp.StatusCode)
	}

	// KV-v2 response shape: { "data": { "data": {...fields...}, "metadata": {...} } }
	var body struct {
		Data struct {
			Data map[string]interface{} `json:"data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("vault decode %s: %w", path, err)
	}
	raw, ok := body.Data.Data[field]
	if !ok {
		return "", fmt.Errorf("vault: field %q not present at %s", field, path)
	}
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("vault: field %q at %s is not a string", field, path)
	}
	// Defense against trailing newlines / whitespace introduced when a
	// secret is written via `<<<` heredocs or `echo` instead of `printf`.
	// FRED-style query-param APIs reject the encoded %0A and return 400.
	return strings.TrimSpace(s), nil
}

// splitMountPath splits a KV path into (mount, key). The mount is the
// first segment; the key is everything after. Both halves required.
//
//	"secret/go-trader/fred"  → ("secret", "go-trader/fred")
//	"kv/foo"                  → ("kv", "foo")
//	"oneSegment"              → ("", "")
//	""                        → ("", "")
func splitMountPath(p string) (mount, key string) {
	p = strings.Trim(p, "/")
	idx := strings.Index(p, "/")
	if idx < 0 {
		return "", ""
	}
	return p[:idx], p[idx+1:]
}
