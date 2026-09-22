package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// 本文件是 CheckpointManager 的**文件实现**（规格 §8.2「权威状态的写入纪律」）。
//
// 【它替掉内存实现的什么】内存实现（backend.go 里的 memCheckpoints）把内容放在
// Backend.durable 里，daemon 一重启就没了 —— 而用户看到的是同一个会话，表现为
// "会话内容莫名缺失"。文件实现把同一份内容写到 <dataDir>/checkpoints/，重启后
// 由同一 dataDir 的新 Backend 读回。
//
// 【写入顺序，逐条对应 §8.2】
//
//	临时文件 → flush → fsync → atomic rename
//
// 崩溃语义：任何时刻断电，目标文件要么是旧内容、要么是新内容，不存在半截 ——
// 因为目标文件只在 rename 那一刻被整体替换，而 rename 在同一文件系统内是原子的。
//
// 【Windows 上的取舍，必须写清楚】规格 §8.2 的 "fsync" 在 POSIX 语境里含两件事：
// 刷文件数据 + 刷目录项（让新文件名本身落盘）。Windows 上无法把目录当文件句柄
// 打开再 Sync，第二件事无对应语义；文件数据由 tmp.Sync() 保证，文件名落盘交由
// NTFS 的日志与 MoveFileEx 的原子替换语义承担。因此本实现在 rename 之后**不**再
// 对目录做 fsync —— 这不是漏做，是该平台上唯一有意义的等价写法。

// ErrCheckpointCorrupt 表示 checkpoint 文件存在，但内容不是本实现写下的那一份
// （信封 magic / 长度 / 摘要任一对不上）。
//
// 【为什么要一个专门的错误值】"文件损坏"和"文件不存在"必须能被区分：前者要按
// 规格 §8.2 转入明确的 CORRUPTED 恢复分支（提示用户、尝试 journal），后者只是
// "从来没写过"。把两者都当成"没有 checkpoint"，就会让损坏静默退化成"从空开始"，
// 正是规格点名禁止的形态。
var ErrCheckpointCorrupt = errors.New("runtime: checkpoint 文件损坏")

// checkpointMagic 是信封开头的魔数，用来把"我们的文件"与"别人写的/残留的文件"
// 分开 —— 只看文件扩展名会让任何同名垃圾文件被当成合法 checkpoint 解析。
const checkpointMagic = "PKGCKPT1"

// checkpointHeaderLen = magic(8) + payload 长度(8) + sha256 摘要(32)。
const checkpointHeaderLen = len(checkpointMagic) + 8 + sha256.Size

// fileCheckpoints 自身不持有内存副本，只按 key 算路径；并发控制由它引用的
// mu（Backend.ckptMu，进程内每后端一把读写锁）承担。
//
// 【为什么读也要参与互斥 —— 这是实测出来的，不是保险】Windows 上 os.Rename 走
// MoveFileEx(REPLACE_EXISTING)，而 Go 的 syscall.Open 只授 FILE_SHARE_READ|
// FILE_SHARE_WRITE（无 FILE_SHARE_DELETE）。因此只要有读者还开着目标文件，
// 写者的替换就返回 ERROR_ACCESS_DENIED。注意这是**读者阻塞写者**：先把写者串成
// 互斥后失败依旧复现，直到把读也纳入同一把读写锁才消失。语义上不损失任何东西
// —— 读者之间仍并行，写者等到无人持有文件时再替换。
//
// 锁放在 Backend 上而不是这里，是因为本类型每次 Checkpoints() 调用都会新建一个
// 实例（对象内锁等于没锁），同一后端的所有入口必须共享同一把。
type fileCheckpoints struct {
	dir string // <dataDir>/checkpoints
	mu  *sync.RWMutex
}

var _ CheckpointManager = (*fileCheckpoints)(nil)

// checkpointsDir 是 checkpoint 的落盘目录。
func checkpointsDir(dataDir string) string { return filepath.Join(dataDir, "checkpoints") }

// Save 原子覆盖写。失败路径会清掉自己创建的临时文件。
func (c *fileCheckpoints) Save(_ context.Context, key CheckpointKey, payload []byte) error {
	if err := key.Validate(); err != nil {
		return err
	}
	// 写路径独占（原因见 fileCheckpoints 的注释）。锁粒度覆盖到 MkdirAll 是有意的：
	// 目录创建本身在并发下也可能互相撞车，且它只在首次写入时真正做事。
	if c.mu != nil {
		c.mu.Lock()
		defer c.mu.Unlock()
	}
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return fmt.Errorf("runtime: 建 checkpoint 目录失败(%s): %w", c.dir, err)
	}

	final := c.pathFor(key)
	// 临时文件必须与目标**同目录**：rename 的原子性以"同一文件系统"为前提。跨文件
	// 系统时 rename 不会"退化成拷贝"，而是直接失败（POSIX 的 EXDEV / Windows 的
	// ERROR_NOT_SAME_DEVICE），原子性根本无从谈起。
	tmp, err := os.CreateTemp(c.dir, "ckpt-*.tmp")
	if err != nil {
		return fmt.Errorf("runtime: 建临时 checkpoint 失败: %w", err)
	}
	tmpName := tmp.Name()
	// 任何失败路径都要清掉临时文件：残留的 .tmp 会让"目录里有什么"不再可信，
	// 也让下一次恢复扫描把垃圾当成候选快照。
	fail := func(err error) error {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}

	if _, err := tmp.Write(encodeCheckpoint(payload)); err != nil {
		return fail(fmt.Errorf("runtime: 写临时 checkpoint 失败: %w", err))
	}
	// flush + fsync：Sync 落在**文件句柄**上，把数据与长度刷到持久介质。
	// 目录项不单独 fsync 的原因见文件头的 Windows 取舍说明。
	if err := tmp.Sync(); err != nil {
		return fail(fmt.Errorf("runtime: fsync 临时 checkpoint 失败: %w", err))
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("runtime: 关闭临时 checkpoint 失败: %w", err)
	}
	// atomic rename：目标文件在此刻被整体替换。Go 在 Windows 上走
	// MoveFileEx(..., MOVEFILE_REPLACE_EXISTING)，已存在的目标会被替换而非报错，
	// 因此"覆盖写"与 POSIX 语义一致。
	if err := os.Rename(tmpName, final); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("runtime: 原子替换 checkpoint 失败(%s): %w", key, err)
	}
	return nil
}

// Load 读回。文件不存在 = 明确的"没有 checkpoint"（found=false, err=nil）；
// 文件损坏 = fail closed 的报错，且**不改写**该文件。
func (c *fileCheckpoints) Load(_ context.Context, key CheckpointKey) ([]byte, bool, error) {
	if err := key.Validate(); err != nil {
		return nil, false, err
	}
	// 读与写互斥（原因见 fileCheckpoints 的注释）：不为保护内存态（这里没有内存态），
	// 而是因为 Windows 上"读者还开着文件"会让写者的 replace 失败。读之间仍可并行。
	if c.mu != nil {
		c.mu.RLock()
		defer c.mu.RUnlock()
	}
	path := c.pathFor(key)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// 契约语义：从未写过不是错误。把它当错误会让"新会话首次恢复"变成
			// 一条误导性告警，调用方还得在正确的路径上吞掉它。
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("runtime: 读 checkpoint 失败(%s): %w", key, err)
	}
	payload, ok := decodeCheckpoint(raw)
	if !ok {
		// fail closed：损坏就是损坏 —— 既不按空/部分内容返回（那会让调用方
		// 拿到一份"看着成功、实际残缺"的状态），也不顺手用空内容把原文件覆盖掉
		// （那会把"可能可人工修复的损坏"变成永久丢失）。
		return nil, false, fmt.Errorf("%w: %s（文件: %s）", ErrCheckpointCorrupt, key, path)
	}
	return payload, true, nil
}

// Drop 删除。删不存在的 checkpoint 视为成功（幂等：调用点常在 defer 里）。
func (c *fileCheckpoints) Drop(_ context.Context, key CheckpointKey) error {
	if err := key.Validate(); err != nil {
		return err
	}
	// 删除同样要独占：Windows 上删除一个正被读者打开的文件会失败（同一条句柄共享限制）。
	if c.mu != nil {
		c.mu.Lock()
		defer c.mu.Unlock()
	}
	if err := os.Remove(c.pathFor(key)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("runtime: 删 checkpoint 失败(%s): %w", key, err)
	}
	return nil
}

// pathFor 把 key 映射到一个文件名。
//
// 【为什么用哈希定名，而不是把 agent/thread/name 直接拼进路径】这三个字段来自
// provider 申报与 store，若直接拼接，一个含 ".." 或路径分隔符的 id 就能把写入引到
// 数据目录之外（路径穿越）。哈希定名让这件事在结构上不可能发生。保留可读前缀只是
// 为排障（打开目录能看出是谁的 checkpoint），唯一性由哈希后缀负责 —— 前缀会被
// 替换字符，本身不保证唯一。
func (c *fileCheckpoints) pathFor(key CheckpointKey) string {
	sum := sha256.Sum256([]byte(key.String()))
	name := fmt.Sprintf("%s.%s.%s-%s.ckpt",
		checkpointFileComponent(key.AgentID),
		checkpointFileComponent(key.ThreadID),
		checkpointFileComponent(key.Name),
		hex.EncodeToString(sum[:8]))
	return filepath.Join(c.dir, name)
}

// checkpointFileComponent 把 key 的一个字段裁剪成"可以安全用作文件名"的片段：
// 只保留 ASCII 字母数字与 - _ .，其余替换为 '_'，并限长。
//
// 【为什么不直接拒绝非法字符】checkpoint 的可用性不该因为一个 id 里带了非 ASCII
// 就整体失效 —— 那是把命名规范问题升级成功能缺失。替换 + 哈希后缀保证既可用又唯一。
func checkpointFileComponent(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			sb.WriteRune(r)
		default:
			sb.WriteByte('_')
		}
	}
	out := sb.String()
	if len(out) > 48 { // 仅 ASCII，字节==字符
		out = out[:48]
	}
	// "" 与 "." / ".." 都是文件系统里的特殊名字，替换掉以免生成意外路径。
	if out == "" || out == "." || out == ".." {
		out = "_"
	}
	return out
}

// encodeCheckpoint 给不透明 payload 套一层信封：
//
//	magic(8) | payloadLen(8, big-endian) | sha256(payload)(32) | payload
//
// 【为什么需要信封】CheckpointManager 的 payload 是**不透明字节**（由调用方自己
// 序列化）。裸写时，"文件被截断"与"合法的短 payload"在字节层面无法区分 —— 那就
// 无从 fail closed。信封把"这份内容是不是我们写下的那一份"变成可判定的。
func encodeCheckpoint(payload []byte) []byte {
	sum := sha256.Sum256(payload)
	buf := make([]byte, 0, checkpointHeaderLen+len(payload))
	buf = append(buf, checkpointMagic...)
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(len(payload)))
	buf = append(buf, n[:]...)
	buf = append(buf, sum[:]...)
	buf = append(buf, payload...)
	return buf
}

// decodeCheckpoint 解信封。任一处对不上都返回 ok=false（由调用方翻成 fail-closed
// 错误）。返回值是 payload 的副本 —— 调用方可以自由改写，不会回头碰到共享缓冲。
func decodeCheckpoint(raw []byte) ([]byte, bool) {
	if len(raw) < checkpointHeaderLen {
		return nil, false
	}
	if string(raw[:len(checkpointMagic)]) != checkpointMagic {
		return nil, false
	}
	declared := binary.BigEndian.Uint64(raw[len(checkpointMagic) : len(checkpointMagic)+8])
	out := raw[checkpointHeaderLen:]
	if declared != uint64(len(out)) {
		return nil, false
	}
	want := raw[len(checkpointMagic)+8 : checkpointHeaderLen]
	sum := sha256.Sum256(out)
	if !bytes.Equal(sum[:], want) {
		return nil, false
	}
	return append([]byte(nil), out...), true
}
