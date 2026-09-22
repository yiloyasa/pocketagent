package server

// 设备注册表(P0-2):每设备独立 token + 挑战应答,支持撤销。
//
// 【讲解】旧模型是"单 token 全设备共用":泄漏一处就得全员换锁。
// 现在每个手机配对时登记 (device_id, P-256 公钥),领一份只属于自己的
// 随机 token(daemon 只存它的 SHA-256);token 失效时手机可用 Keystore
// 私钥对挑战数签名续期;丢了/换机直接 Revoke 删条目。
// 旧手机(只有单 token)继续兼容 —— validate 查不到设备时回退比对
// 旧 token,命中即视为 legacy 设备,不强制迁移。

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// 挑战数有效期:手机拿到挑战后几秒内就会应答,5 分钟是宽裕的上限。
const challengeTTL = 5 * time.Minute

// deviceRecord 是 devices.json 里的一台已配对设备。
// pub_key 存 base64(SPKI DER)的 P-256 公钥,与手机端 Keystore 导出格式一致。
type deviceRecord struct {
	DeviceID     string   `json:"device_id"`
	Name         string   `json:"name"`
	PubKey       string   `json:"pub_key"`
	TokenHash    string   `json:"token_hash"` // sha256(token) hex,绝不存明文
	Scopes       []string `json:"scopes"`
	CreatedUnix  int64    `json:"created_unix"`
	LastSeenUnix int64    `json:"last_seen_unix"`
}

// pendingChallenge 是一次已签发、尚未验证的挑战。
type pendingChallenge struct {
	Value     string
	ExpiresAt time.Time
}

type deviceRegistry struct {
	mu          sync.Mutex
	path        string // devices.json 路径;空 = 仅内存(单测用)
	pairCode    string // 原始配对码(注册新设备时比对用)
	legacyToken string // 旧单 token 的期望值(= sha256("pair-code:"+配对码) 的 hex),兼容存量手机
	devices     map[string]*deviceRecord
	challenges  map[string]*pendingChallenge
	nowFunc     func() time.Time // 可注入时钟(单测用过期路径)
	randomFunc  func([]byte) (int, error)
	degraded    bool  // 【规格 §8.2】设备表损坏(JSON 非法/条目残缺)→ 降级态:暂停远程认证
	degradedErr error // 触发降级的原因,供日志与诊断
}

func newDeviceRegistry(dataDir, pairCode string) *deviceRegistry {
	r := &deviceRegistry{
		path:        "",
		pairCode:    pairCode,
		legacyToken: legacyTokenHash(pairCode),
		devices:     map[string]*deviceRecord{},
		challenges:  map[string]*pendingChallenge{},
		nowFunc:     time.Now,
		randomFunc:  rand.Read,
	}
	if dataDir != "" {
		r.path = filepath.Join(dataDir, "devices.json")
	}
	if r.path == "" {
		return r
	}
	raw, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return r // 首次启动:还没有设备表,正常空表,不是损坏
		}
		// 设备表存在却读不出来(权限/IO):同样不能信任认证,进入降级态。
		r.markDegraded(fmt.Errorf("读取设备表失败: %w", err))
		return r
	}
	var list []*deviceRecord
	if err := json.Unmarshal(raw, &list); err != nil {
		// 【规格 §8.2】设备表损坏时绝不静默按空表启动 —— 那会让用户误以为
		// 所有设备都消失了(远程认证全部失败却看不出原因)。改为进入降级态:
		// 暂停远程认证,原文件原样保留,日志给出备份/重新配对的人工恢复路径。
		r.markDegraded(fmt.Errorf("设备表 JSON 解析失败: %w", err))
		return r
	}
	for _, d := range list {
		if d == nil || d.DeviceID == "" {
			// 存在无法识别的条目 = 表被截断或改写,同样按损坏处理:否则会
			// 悄悄少掉设备,正是规格要防的"设备凭空消失"。
			r.markDegraded(fmt.Errorf("设备表含无法解析的设备条目(缺 device_id)"))
			continue
		}
		r.devices[d.DeviceID] = d
	}
	if r.degraded {
		return r
	}
	// 【启动备份】每次成功加载,把刚读到的好版本留一份为 devices.json.bak
	// (与 store 的 threads.json.bak 同款做法),供日后损坏时人工恢复。
	// 刻意不自动恢复:把旧数据盖回新数据是产品决策,这里只留证据与路径。
	r.writeBackupAtomic(r.bakPath(), raw)
	return r
}

// degradedAuthMsg 是降级态下返回给客户端的提示:人话,且指明主机侧的恢复路径。
const degradedAuthMsg = "daemon 设备表损坏,已暂停所有设备认证(防止误报'设备全部消失');请在主机上查看日志,用 devices.json.bak 恢复或让手机重新配对"

// markDegraded 把注册表置入降级态:设备表加载失败,暂停远程认证。
// 用醒目 ERROR 日志讲清发生了什么与怎么恢复 —— 绝不静默。
func (r *deviceRegistry) markDegraded(err error) {
	r.degraded = true
	r.degradedErr = err
	log.Printf("[devices] ERROR: %s", r.degradedMessage())
}

// bakPath 返回设备表备份路径(devices.json.bak);仅内存模式为空。
func (r *deviceRegistry) bakPath() string {
	if r.path == "" {
		return ""
	}
	return r.path + ".bak"
}

// degradedMessage 用人话讲清:出了什么事、为什么远程认证被暂停、用户怎么恢复。
func (r *deviceRegistry) degradedMessage() string {
	recovery := "请人工检查原文件"
	if r.path != "" {
		if st, statErr := os.Stat(r.bakPath()); statErr == nil && !st.IsDir() {
			recovery = fmt.Sprintf("可用备份手动恢复:把 %s 复制回 %s;或让手机用配对码重新配对", r.bakPath(), r.path)
		} else {
			recovery = fmt.Sprintf("没有可用备份,请检查原文件 %s;或让手机用配对码重新配对", r.path)
		}
	}
	return fmt.Sprintf("设备表损坏,已暂停所有远程设备认证(fail-closed,防止把\"设备全部消失\"误报给用户)| 原因: %v | 设备表: %s | 恢复: %s",
		r.degradedErr, r.path, recovery)
}

// writeBackupAtomic 把 data 原子地写为 bak(tmp+rename),避免半截备份反过来
// 毒害恢复。备份失败只告警不阻塞 —— 它是恢复的辅助证据,绝不能变成新的
// "写入即死"点。
func (r *deviceRegistry) writeBackupAtomic(bak string, data []byte) {
	if bak == "" {
		return
	}
	tmp := bak + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		log.Printf("[devices] WARNING: 设备表备份写入失败(不影响运行,仅影响损坏后的恢复): %v", err)
		return
	}
	if err := os.Rename(tmp, bak); err != nil {
		log.Printf("[devices] WARNING: 设备表备份落位失败(不影响运行,仅影响损坏后的恢复): %v", err)
	}
}

// backupCurrentLocked 把当前磁盘上的 devices.json(即将被覆盖的上一版)
// 留一份为 .bak,供日后损坏时恢复。文件不存在(首次写入)或读不出来则跳过。
func (r *deviceRegistry) backupCurrentLocked() {
	if r.path == "" {
		return
	}
	cur, err := os.ReadFile(r.path)
	if err != nil {
		return
	}
	r.writeBackupAtomic(r.bakPath(), cur)
}

// legacyTokenHash 与旧 server.token() 完全一致:sha256("pair-code:"+配对码)。
// 【兼容承诺】老版本手机里存的就是这个值,升级 daemon 后必须继续可用。
func legacyTokenHash(pairCode string) string {
	if pairCode == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("pair-code:" + pairCode))
	return hex.EncodeToString(sum[:])
}

// LegacyTokenFor 返回配对码对应的 legacy 单 token 明文值(只读 helper,
// T-L2fix):= sha256("pair-code:"+配对码) 的 hex,与 deviceRegistry.Validate
// 的 legacy 回退路径所认的值完全一致。供 cmd/pocketagentd 的 -pi-collab 接线
// 取值 —— 写进 mcp.json 的 env(键 POCKETAGENT_TOKEN),让本机 -collab-mcp
// 代理进程拿它作为 Bearer token 回环调用主 daemon(Validate 的 legacy 回退
// 认此值,见 authorized → Validate)。
// 配对码为空返回空串(调用方按"无 token 可给"处理,不注入 env)。
func LegacyTokenFor(pairCode string) string {
	return legacyTokenHash(pairCode)
}

// saveLocked 整体写回 devices.json,沿用 store 的 .tmp+rename 原子写法。
func (r *deviceRegistry) saveLocked() error {
	if r.path == "" {
		return nil // 仅内存模式
	}
	list := make([]*deviceRecord, 0, len(r.devices))
	for _, d := range r.devices {
		list = append(list, d)
	}
	raw, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	// 【写入前备份】覆盖主文件前,把上一版留为 devices.json.bak。但降级态
	// (degraded)下磁盘上的主文件本身就是坏的,绝不能拿它覆盖可能还有价值
	// 的旧 .bak —— 那正是恢复的最后一条路。
	if !r.degraded {
		r.backupCurrentLocked()
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, r.path); err != nil {
		return err
	}
	// 成功写回即代表设备表已重建/可信,解除降级(例如重新配对恢复路径)。
	r.degraded = false
	r.degradedErr = nil
	return nil
}

// newToken 生成 64 位十六进制随机 token(32 字节熵)。
func (r *deviceRegistry) newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := r.randomFunc(b); err != nil {
		return "", fmt.Errorf("生成随机 token 失败: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// hashToken 返回 token 的 sha256 hex —— 存储与比较都用它,不碰明文。
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// RegisterByPairCode 校验配对码并注册设备,返回该设备专属的新 token。
// 已存在的 device_id 允许重新配对(换机重装后公钥会变,以最新一次为准)。
func (r *deviceRegistry) RegisterByPairCode(pairCode, deviceID, name, pubKey string) (string, error) {
	if deviceID == "" || pubKey == "" {
		return "", errInvalidArg("缺少 device_id 或 device_pub_key")
	}
	// 先验公钥格式,把坏请求挡在落盘之前。
	if _, err := parsePubKey(pubKey); err != nil {
		return "", errInvalidArg("device_pub_key 不是合法的 P-256 公钥: " + err.Error())
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if subtle.ConstantTimeCompare([]byte(pairCode), []byte(r.pairCode)) != 1 {
		return "", errPermissionDenied("配对码错误")
	}
	now := r.nowFunc().Unix()
	rec := r.devices[deviceID]
	if rec == nil {
		rec = &deviceRecord{DeviceID: deviceID, CreatedUnix: now}
		r.devices[deviceID] = rec
	}
	rec.Name = name
	rec.PubKey = pubKey
	rec.Scopes = []string{"all"}
	rec.LastSeenUnix = now
	token, err := r.newToken()
	if err != nil {
		return "", err
	}
	rec.TokenHash = hashToken(token)
	if err := r.saveLocked(); err != nil {
		return "", errInternal(err)
	}
	return token, nil
}

// IssueChallenge 给指定设备发一个 32 字节随机挑战数(hex),缓存待验。
// 设备未注册时拒绝 —— 挑战只发给已知公钥的设备。
func (r *deviceRegistry) IssueChallenge(deviceID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.degraded {
		// fail-closed:设备表不可信时不签发任何挑战(认证链的第一步就挡住)。
		return "", errUnavailable(degradedAuthMsg)
	}
	if r.devices[deviceID] == nil {
		return "", errNotFound("设备未注册,请先用配对码配对")
	}
	r.gcChallengesLocked()
	b := make([]byte, 32)
	if _, err := r.randomFunc(b); err != nil {
		return "", errInternal(err)
	}
	ch := hex.EncodeToString(b)
	r.challenges[deviceID] = &pendingChallenge{Value: ch, ExpiresAt: r.nowFunc().Add(challengeTTL)}
	return ch, nil
}

// VerifyChallenge 验证 "device_id:challenge" 上的 P-256 签名,
// 通过后给该设备签发新 token(旧 token 立即作废)。
func (r *deviceRegistry) VerifyChallenge(deviceID, challenge, sigB64 string) (string, error) {
	if sigB64 == "" {
		return "", errInvalidArg("缺少签名")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.degraded {
		// fail-closed:设备表不可信时不接受任何挑战应答(与 IssueChallenge 对应)。
		return "", errUnavailable(degradedAuthMsg)
	}
	rec := r.devices[deviceID]
	if rec == nil {
		return "", errNotFound("设备未注册,请先用配对码配对")
	}
	pc := r.challenges[deviceID]
	// 【先消费再验证】无论验证成败,挑战一律作废:防重放,也避免半失效
	// 的挑战被反复尝试。
	if pc == nil || pc.Value != challenge || r.nowFunc().After(pc.ExpiresAt) {
		delete(r.challenges, deviceID)
		return "", errPermissionDenied("挑战无效或已过期")
	}
	delete(r.challenges, deviceID)
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return "", errInvalidArg("签名不是合法 base64")
	}
	pub, err := parsePubKey(rec.PubKey)
	if err != nil {
		return "", errInternal(err)
	}
	sum := sha256.Sum256([]byte(deviceID + ":" + challenge))
	// SHA256withECDSA(手机 Keystore)输出 ASN.1 DER,对应 VerifyASN1。
	if !ecdsa.VerifyASN1(pub, sum[:], sig) {
		return "", errPermissionDenied("挑战签名验证失败")
	}
	token, err := r.newToken()
	if err != nil {
		return "", err
	}
	rec.TokenHash = hashToken(token)
	rec.LastSeenUnix = r.nowFunc().Unix()
	if err := r.saveLocked(); err != nil {
		return "", errInternal(err)
	}
	return token, nil
}

// Revoke 撤销设备:删条目即失效,其 token 下一次请求起被拒。
func (r *deviceRegistry) Revoke(deviceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.devices[deviceID] == nil {
		return errNotFound("设备不存在")
	}
	delete(r.devices, deviceID)
	delete(r.challenges, deviceID)
	if err := r.saveLocked(); err != nil {
		return errInternal(err)
	}
	return nil
}

// Validate 校验 Bearer token:先查设备表(常时比较),再回退旧单 token。
// 返回设备 ID;legacy=true 表示命中的是兼容路径的旧 token。
// 【常时比较】逐个 hash 用 subtle 比较,避免时序侧信道泄露"前缀对不对"。
func (r *deviceRegistry) Validate(token string) (deviceID string, legacy, ok bool) {
	if token == "" {
		return "", false, false
	}
	hash := hashToken(token)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.degraded {
		// fail-closed(规格 §8.2):设备表不可信时,任何 token(含 legacy 回退)
		// 都不能通过 —— 宁可拒绝并要求重新配对,也不让用户以为"所有设备都
		// 消失了"。恢复后(重新配对写回)自动解除降级。
		return "", false, false
	}
	for id, d := range r.devices {
		if subtle.ConstantTimeCompare([]byte(hash), []byte(d.TokenHash)) == 1 {
			// last_seen 节流刷新(1 分钟内不重复落盘):Validate 挂在每个请求的
			// 鉴权路径上,每次都整体写回 devices.json 会把磁盘当心跳打。
			if now := r.nowFunc().Unix(); now-d.LastSeenUnix >= 60 {
				d.LastSeenUnix = now
				_ = r.saveLocked()
			}
			return id, false, true
		}
	}
	// legacy 回退:旧拦截器是拿 Bearer 原文直接比 sha256("pair-code:"+code)
	// 的 hex 值,这里保持同一语义(不能对原文再哈希一次)。
	if r.legacyToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(r.legacyToken)) == 1 {
		return "legacy", true, true
	}
	return "", false, false
}

// gcChallengesLocked 清掉过期挑战(调用方需持锁)。
func (r *deviceRegistry) gcChallengesLocked() {
	now := r.nowFunc()
	for id, pc := range r.challenges {
		if now.After(pc.ExpiresAt) {
			delete(r.challenges, id)
		}
	}
}

// parsePubKey 把 base64(SPKI DER) 解成 P-256 公钥。
func parsePubKey(pubB64 string) (*ecdsa.PublicKey, error) {
	der, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil {
		return nil, fmt.Errorf("base64 解码失败: %w", err)
	}
	generic, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("SPKI 解析失败: %w", err)
	}
	pub, ok := generic.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("期望 ECDSA 公钥,得到 %T", generic)
	}
	if pub.Curve != elliptic.P256() {
		return nil, fmt.Errorf("期望 P-256 曲线,得到 %s", pub.Curve.Params().Name)
	}
	return pub, nil
}
