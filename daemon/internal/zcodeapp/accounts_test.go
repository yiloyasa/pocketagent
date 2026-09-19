package zcodeapp

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func encryptedFixture(t *testing.T, value, secret string) string {
	t.Helper()
	key := sha256.Sum256([]byte(secret))
	block, _ := aes.NewCipher(key[:])
	gcm, _ := cipher.NewGCM(block)
	iv := make([]byte, gcm.NonceSize())
	out := gcm.Seal(nil, iv, []byte(value), nil)
	enc := base64.RawURLEncoding.EncodeToString
	return "enc:v1:" + enc(iv) + "." + enc(out[len(out)-gcm.Overhead():]) + "." + enc(out[:len(out)-gcm.Overhead()])
}

func TestDesktopAccountsNativeOverlayAndAuth(t *testing.T) {
	root := t.TempDir()
	builtin := filepath.Join(root, "builtin.json")
	write := func(name string, v any) {
		t.Helper()
		raw, _ := json.Marshal(v)
		if err := os.WriteFile(filepath.Join(root, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("builtin.json", map[string]any{"revision": 42, "config": map[string]any{"providerConfigRules": map[string]any{"providerRules": []any{
		map[string]any{"providerId": "account:future-provider", "config": map[string]any{"builtinModelIds": []string{"future-not-hardcoded"}, "access": map[string]string{"type": "zhipu-account", "accountType": "bigmodel", "mode": "individual-coding-plan"}}},
	}}}})
	write("setting.json", map[string]any{"providerFamilyDomain": "bigmodel", "providerFamilyConnectionSelections": map[string]any{"bigmodel": map[string]string{"kind": "individual-coding-plan"}}})
	write("credentials.json", map[string]string{
		"oauth:active_provider":    encryptedFixture(t, "bigmodel", "test-secret"),
		"oauth:bigmodel:user_info": encryptedFixture(t, `{"id":"current-user"}`, "test-secret"),
		"account-provider:coding-plan:account:future-provider:account:current-user:api-key": encryptedFixture(t, "test-key", "test-secret"),
		"account-provider:coding-plan:account:future-provider:account:other-user:api-key":   encryptedFixture(t, "wrong-key", "test-secret"),
	})
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/api/biz/subscription/list" || r.Header.Get("Authorization") != "test-key" {
			t.Error("incorrect subscription request")
		}
		_, _ = w.Write([]byte(`{"code":200,"success":true,"data":[{"productName":"Coding Plan","status":"VALID","inCurrentPeriod":true}]}`))
	}))
	defer srv.Close()
	a := &desktopAccounts{root: root, builtin: builtin, secret: "test-secret", origins: map[string]string{"bigmodel": srv.URL}, http: srv.Client()}
	snap, err := a.read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !snap.States["account:future-provider"].Entitled || !snap.States["account:future-provider"].Current {
		t.Fatalf("not entitled: %+v", snap.States)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls=%d", calls.Load())
	}
	client, _ := newTestClient()
	transport := &accountReceiptWriter{client: client}
	client.stdin = transport
	if err := a.sync(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	checkedCalls := calls.Load()
	if err := a.sync(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	if len(transport.snapshots) != 1 || calls.Load() != checkedCalls {
		t.Fatal("unchanged source was re-published/refetched within TTL")
	}
	key, err := a.requestKey("account:future-provider", "bigmodel", "individual-coding-plan")
	if err != nil || key != "test-key" {
		t.Fatalf("auth error=%v, correct key=%t", err, key == "test-key")
	}
	// Logout must revoke a previously usable key without restarting the client.
	write("credentials.json", map[string]string{})
	if _, err := a.requestKey("account:future-provider", "bigmodel", "individual-coding-plan"); err == nil {
		t.Fatal("stale key used after logout")
	}
	snap, err = a.read(context.Background())
	if err != nil || snap.States["account:future-provider"].Entitled {
		t.Fatalf("logout did not revoke overlay: %v", err)
	}
	if err := a.sync(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	if len(transport.snapshots) != 2 || transport.snapshots[1].States["account:future-provider"].Entitled {
		t.Fatal("logout did not publish revoked overlay immediately")
	}
}

type accountReceiptWriter struct {
	client    *Client
	snapshots []accountSnapshot
}

func (w *accountReceiptWriter) Write(raw []byte) (int, error) {
	var req struct {
		ID     json.RawMessage `json:"id"`
		Params accountSnapshot `json:"params"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return 0, err
	}
	w.snapshots = append(w.snapshots, req.Params)
	reply, _ := json.Marshal(map[string]string{"receivedRevision": req.Params.Revision})
	w.client.dispatch(&wireMsg{ID: req.ID, Result: reply})
	return len(raw), nil
}
func (w *accountReceiptWriter) Close() error { return nil }

func TestCredentialCipherRejectsWrongSecretWithoutLeaking(t *testing.T) {
	value := encryptedFixture(t, "never-print-this", "correct")
	plain, err := decryptDesktopCredential(value, "wrong")
	if err == nil || plain != "" {
		t.Fatal("wrong secret accepted")
	}
}
