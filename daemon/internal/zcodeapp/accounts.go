package zcodeapp

// The desktop host (out/host/index.js, 3.14.0) delivers an account overlay to
// app-server and answers interaction/requestProviderRuntimeHeaders. Merely
// supplying builtin/personal JSON never loads account providers in that worker.
//
// This read-only adapter follows the installed host's individual Coding Plan
// path: current family/profile -> scoped cached API key -> subscription/list
// entitlement -> provider/updateAccountConfig. Models/reasoning remain entirely
// native builtin + personal config; no model catalog is manufactured here.
// Team/Start/Off-peak require different native auth flows and are not inferred
// from an individual-plan credential. No credential/config writes or OAuth
// refresh are performed.

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type accountState struct {
	Availability string `json:"availability"`
	Entitled     bool   `json:"entitled"`
	Current      bool   `json:"current"`
}

// AccountSyncError is not a broken transport; callers must not restart an agent
// or create a replacement conversation in response to this failure.
type AccountSyncError struct{ Err error }

func (e *AccountSyncError) Error() string { return "ZCode desktop account sync: " + e.Err.Error() }
func (e *AccountSyncError) Unwrap() error { return e.Err }

type accountSnapshot struct {
	Revision        string                  `json:"revision"`
	BuiltinRevision string                  `json:"basedOnZCodeBuiltinRevision"`
	Providers       map[string]any          `json:"providers"`
	States          map[string]accountState `json:"states"`
}

type accountRule struct {
	ProviderID string `json:"providerId"`
	Config     struct {
		Access struct {
			Type   string `json:"type"`
			Mode   string `json:"mode"`
			Family string `json:"accountType"`
		} `json:"access"`
	} `json:"config"`
}

type desktopAccounts struct {
	root, builtin, secret string
	origins               map[string]string
	http                  *http.Client
	mu                    sync.Mutex
	published             string
	checkedAt             time.Time
	fingerprint           [32]byte
}

func newDesktopAccounts(env []string) *desktopAccounts {
	get := func(key string) string {
		for _, pair := range env {
			k, v, ok := strings.Cut(pair, "=")
			if ok && strings.EqualFold(k, key) {
				return strings.TrimSpace(v)
			}
		}
		return ""
	}
	home, _ := os.UserHomeDir()
	desktopHome := get("ZCODE_DESKTOP_HOME_DIR")
	if desktopHome == "" {
		desktopHome = home
	}
	root := filepath.Join(desktopHome, ".zcode", "v2")
	secret := get("ZCODE_CREDENTIAL_SECRET")
	if secret == "" {
		username := "unknown"
		if u, err := user.Current(); err == nil {
			username = u.Username
			if i := strings.LastIndex(username, `\`); i >= 0 {
				username = username[i+1:]
			}
		}
		platform := runtime.GOOS
		if platform == "windows" {
			platform = "win32"
		}
		secret = "zcode-credential-fallback:" + platform + ":" + home + ":" + username
	}
	envName := "PRODUCTION"
	bigmodel, zai := "https://bigmodel.cn", "https://api.z.ai"
	if get("ZCODE_ENV") == "test" {
		envName = "TEST"
		bigmodel, zai = "https://dev.bigmodel.cn", "https://api.chatglm.site"
	}
	origin := func(generic, scoped, fallback string) string {
		if v := get(generic); v != "" {
			return v
		}
		if v := get(scoped); v != "" {
			return v
		}
		return fallback
	}
	return &desktopAccounts{
		root: root, builtin: get(zcodeEnvBuiltin), secret: secret,
		origins: map[string]string{
			"bigmodel": origin("BIGMODEL_API_BASE_URL", "BIGMODEL_"+envName+"_API_BASE_URL", bigmodel),
			"zai":      origin("ZAI_BUSINESS_BASE_URL", "ZAI_"+envName+"_BUSINESS_BASE_URL", zai),
		},
		http: &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}
}

func decryptDesktopCredential(value, secret string) (string, error) {
	if !strings.HasPrefix(value, "enc:v1:") {
		return value, nil
	}
	parts := strings.Split(strings.TrimPrefix(value, "enc:v1:"), ".")
	if len(parts) != 3 {
		return "", errors.New("ZCode credential ciphertext 格式错误")
	}
	decoded := make([][]byte, 3)
	for i, p := range parts {
		v, e := base64.RawURLEncoding.DecodeString(p)
		if e != nil {
			return "", errors.New("ZCode credential ciphertext 编码错误")
		}
		decoded[i] = v
	}
	key := sha256.Sum256([]byte(secret))
	block, _ := aes.NewCipher(key[:])
	gcm, _ := cipher.NewGCM(block)
	if len(decoded[0]) != gcm.NonceSize() || len(decoded[1]) != gcm.Overhead() {
		return "", errors.New("ZCode credential ciphertext 长度错误")
	}
	plain, err := gcm.Open(nil, decoded[0], append(decoded[2], decoded[1]...), nil)
	if err != nil {
		return "", errors.New("ZCode credential 无法解密（未输出凭据）")
	}
	return string(plain), nil
}

type desktopAccountSettings struct {
	Family     string `json:"providerFamilyDomain"`
	Selections map[string]struct {
		Kind string `json:"kind"`
	} `json:"providerFamilyConnectionSelections"`
}

var errAccountNotConnected = errors.New("ZCode 当前账户未连接或缺少该账户的缓存凭据")

func (a *desktopAccounts) identity() (desktopAccountSettings, map[string]string, error) {
	var settings desktopAccountSettings
	if err := readAccountJSON(filepath.Join(a.root, "setting.json"), &settings); err != nil {
		return settings, nil, err
	}
	credentials := map[string]string{}
	if err := readAccountJSON(filepath.Join(a.root, "credentials.json"), &credentials); err != nil {
		return settings, nil, err
	}
	return settings, credentials, nil
}

func readAccountJSON(path string, out any) error {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return errors.New("ZCode account source 无法读取")
	}
	if json.Unmarshal(raw, out) != nil {
		return errors.New("ZCode account source JSON 无效")
	}
	return nil
}

func (a *desktopAccounts) keyFor(settings desktopAccountSettings, credentials map[string]string, provider, family, mode string) (string, error) {
	if settings.Family != family || settings.Selections[family].Kind != mode || mode != "individual-coding-plan" {
		return "", errors.New("ZCode 当前账户连接不匹配，或该账户模式尚未接入")
	}
	active, err := decryptDesktopCredential(credentials["oauth:active_provider"], a.secret)
	if err != nil {
		return "", err
	}
	if active != family {
		return "", errAccountNotConnected
	}
	profile, err := decryptDesktopCredential(credentials["oauth:"+family+":user_info"], a.secret)
	if err != nil {
		return "", err
	}
	var user struct {
		ID     string `json:"id"`
		UserID string `json:"user_id"`
	}
	if strings.TrimSpace(profile) == "" {
		return "", errAccountNotConnected
	}
	if json.Unmarshal([]byte(profile), &user) != nil {
		return "", errors.New("ZCode 当前账户资料不可用")
	}
	id := user.ID
	if id == "" && family == "zai" {
		id = user.UserID
	}
	if id == "" {
		return "", errAccountNotConnected
	}
	// JS encodeURIComponent and PathEscape differ for a few reserved characters.
	encoded := url.QueryEscape(id)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	for _, ch := range []string{"!", "'", "(", ")", "*"} {
		encoded = strings.ReplaceAll(encoded, fmt.Sprintf("%%%02X", ch[0]), ch)
	}
	key, err := decryptDesktopCredential(credentials["account-provider:coding-plan:"+provider+":account:"+encoded+":api-key"], a.secret)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(key) == "" {
		return "", errAccountNotConnected
	}
	return strings.TrimSpace(key), nil
}

func (a *desktopAccounts) requestKey(provider, family, mode string) (string, error) {
	// Read on every native model request: logout / account changes revoke the
	// old material immediately, without waiting for the registry refresh TTL.
	settings, credentials, err := a.identity()
	if err != nil {
		return "", err
	}
	var builtin struct {
		Config struct {
			Rules struct {
				Providers []accountRule `json:"providerRules"`
			} `json:"providerConfigRules"`
		} `json:"config"`
	}
	if err = readAccountJSON(a.builtin, &builtin); err != nil {
		return "", err
	}
	for _, r := range builtin.Config.Rules.Providers {
		if r.ProviderID == provider && r.Config.Access.Type == "zhipu-account" && r.Config.Access.Family == family && r.Config.Access.Mode == mode {
			return a.keyFor(settings, credentials, provider, family, mode)
		}
	}
	return "", errors.New("ZCode 请求账户与 builtin provider 不匹配")
}

func (a *desktopAccounts) read(ctx context.Context) (accountSnapshot, error) {
	out := accountSnapshot{Providers: map[string]any{}, States: map[string]accountState{}}
	var builtin struct {
		Revision int64 `json:"revision"`
		Config   struct {
			Rules struct {
				Providers []accountRule `json:"providerRules"`
			} `json:"providerConfigRules"`
		} `json:"config"`
	}
	if err := readAccountJSON(a.builtin, &builtin); err != nil {
		return out, err
	}
	abs, err := filepath.Abs(a.builtin)
	if err != nil {
		return out, err
	}
	pathHash := sha256.Sum256([]byte(abs))
	out.BuiltinRevision = fmt.Sprintf("zcode-builtin:%d:%x", builtin.Revision, pathHash)
	settings, credentials, err := a.identity()
	if err != nil {
		return out, err
	}
	for _, r := range builtin.Config.Rules.Providers {
		access := r.Config.Access
		if access.Type != "zhipu-account" {
			continue
		}
		state := accountState{Availability: "unavailable"}
		if settings.Family == access.Family && settings.Selections[access.Family].Kind == access.Mode && access.Mode == "individual-coding-plan" {
			key, e := a.keyFor(settings, credentials, r.ProviderID, access.Family, access.Mode)
			if e != nil && !errors.Is(e, errAccountNotConnected) {
				return out, e
			}
			available := false
			if e == nil {
				available, e = a.entitled(ctx, access.Family, key)
			} else {
				e = nil
			}
			if e != nil {
				return out, e
			}
			state.Current = true
			state.Entitled = available
			if available {
				state.Availability = "available"
			}
		}
		out.States[r.ProviderID] = state
		out.Providers[r.ProviderID] = map[string]any{"access": map[string]any{"type": "zhipu-account", "entitled": state.Entitled}}
	}
	raw, _ := json.Marshal(out)
	hash := sha256.Sum256(raw)
	out.Revision = fmt.Sprintf("pocketagent-account:%x", hash)
	return out, nil
}

func (a *desktopAccounts) entitled(ctx context.Context, family, key string) (bool, error) {
	origin := a.origins[family]
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Hostname() != "127.0.0.1") {
		return false, errors.New("ZCode Coding Plan endpoint 无效")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(origin, "/")+"/api/biz/subscription/list", nil)
	if err != nil {
		return false, errors.New("ZCode Coding Plan request 无效")
	}
	req.Header.Set("Authorization", strings.TrimSpace(strings.TrimPrefix(key, "Bearer ")))
	resp, err := a.http.Do(req)
	if err != nil {
		return false, errors.New("ZCode Coding Plan 权益查询失败（网络或超时）")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("ZCode Coding Plan 权益查询 HTTP %d", resp.StatusCode)
	}
	var reply struct {
		Code    *int  `json:"code"`
		Success *bool `json:"success"`
		Data    *[]struct {
			ProductID       string `json:"productId"`
			ProductName     string `json:"productName"`
			Status          string `json:"status"`
			InCurrentPeriod *bool  `json:"inCurrentPeriod"`
		} `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1024*1024)).Decode(&reply) != nil || reply.Data == nil || (reply.Success != nil && !*reply.Success) || (reply.Code != nil && *reply.Code != 0 && *reply.Code != 200) {
		return false, errors.New("ZCode Coding Plan 权益响应无法确认")
	}
	for _, plan := range *reply.Data {
		coding := strings.Contains(strings.ToLower(plan.ProductID), "coding") || strings.Contains(strings.ToLower(plan.ProductName), "coding")
		if coding && plan.Status == "VALID" && plan.InCurrentPeriod != nil && *plan.InCurrentPeriod {
			return true, nil
		}
	}
	return false, nil
}

func (a *desktopAccounts) sourceFingerprint() ([32]byte, error) {
	h := sha256.New()
	for _, p := range []string{a.builtin, filepath.Join(a.root, "setting.json"), filepath.Join(a.root, "credentials.json")} {
		raw, err := os.ReadFile(p)
		if err != nil && !os.IsNotExist(err) {
			return [32]byte{}, errors.New("ZCode account source 无法读取")
		}
		h.Write(raw)
		h.Write([]byte{0})
	}
	var hash [32]byte
	copy(hash[:], h.Sum(nil))
	return hash, nil
}

func (a *desktopAccounts) sync(ctx context.Context, c *Client) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	fingerprint, err := a.sourceFingerprint()
	if err != nil {
		return err
	}
	if a.published != "" && fingerprint == a.fingerprint && time.Since(a.checkedAt) < time.Minute {
		return nil
	}
	snap, err := a.read(ctx)
	if err != nil {
		return err
	}
	after, err := a.sourceFingerprint()
	if err != nil {
		return err
	}
	if after != fingerprint {
		return errors.New("ZCode 账户查询期间配置已变化，请重试")
	}
	if snap.Revision != a.published {
		raw, err := c.CallCtx(ctx, "provider/updateAccountConfig", snap)
		if err != nil {
			return err
		}
		var receipt struct {
			Revision string `json:"receivedRevision"`
		}
		if json.Unmarshal(raw, &receipt) != nil || receipt.Revision != snap.Revision {
			return errors.New("ZCode 账户配置回执 revision 不匹配")
		}
		a.published = snap.Revision
	}
	a.fingerprint = fingerprint
	a.checkedAt = time.Now()
	return nil
}
