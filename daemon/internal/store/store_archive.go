package store

// ============ 消息归档(threads.archive/<threadID>.jsonl)============
//
// 【为什么要有归档】会话消息此前无上限地堆在 threads.json 里:长会话让主文件
// 无限膨胀,每次保存都是全量重写,启动加载也把全部历史读进内存。现在内存与
// 主文件只保留最近 maxMessagesInMemory 条,溢出的最旧条目在每次落库前被**追加**
// 写入旁车文件 <数据目录>/threads.archive/<threadID>.jsonl(JSON Lines,每行一条
// StoredMessage 的 JSON,结构与主文件 messages 数组元素完全一致 —— 未来可手工
// 把归档行拼回主文件 messages 数组头部读回,格式零特殊)。
//
// 【Load 语义】加载不读旁车归档:内存只持有主文件里的消息(上限语义见
// maxMessagesInMemory),归档文件是"更早历史"的落盘留档,按需人工查阅/合并,
// 不参与常规读路径。
//
// 【失败路径(fail-open)】归档写失败(磁盘满/权限/目录不可建)只打 WARNING,
// 不裁剪、不报错、不影响本次保存 —— 宁可下次保存再试归档,也绝不因归档丢消息
// 或让保存失败。极端时序下(归档已落盘、主文件保存前进程崩溃)重启后会从主
// 文件重新读到溢出条目并再归档一次,归档文件可能出现少量重复行 —— 重复无害
// (人工合并时按行去重即可),丢消息不可接受,取舍明确。

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// archiveDirName 是旁车归档目录名,位于 threads.json 所在的数据目录下。
const archiveDirName = "threads.archive"

// DefaultArchiveMaxBytes 归档目录总字节上限的内置默认(32MB)。
// 【口径互指】与 runtime.DefaultLimits 的 DiskConversationBytes(同为 32MB)是
// 同一个语义的两端:生产由 main 在装配期用策略生效值覆盖本默认(SetArchiveMaxBytes,
// 见 cmd/pocketagentd/main.go 的"DiskConversationBytes 接通"),使 disk.conversation
// 从"全仓无执行点的假口径"变成有真实执行体的预算;此处默认值仅覆盖未接线的
// 场景(测试/直接构造 FileStore)。store 不 import runtime,数字各自声明、注释互指。
const DefaultArchiveMaxBytes = 32 << 20

// archiveOverflowLocked 把每个超过 maxMessagesInMemory 条消息的会话的最旧溢出
// 条目追加进旁车归档,成功后把内存裁剪到最近 N 条。必须在 s.mu 持锁下调用
// (saveLocked 的全部调用方均持锁),归档写入因此天然串行,无需文件级锁。
// 逐会话独立处理:某个会话归档失败只跳过该会话,其余照常。
// 【总量守护】本轮确有归档写入时,顺路做一次归档目录总量超限清理(见
// evictArchiveLocked)—— 此前归档只进不出、无读取方,分发机器上无限增长。
func (s *FileStore) archiveOverflowLocked() {
	appended := false
	for _, t := range s.data.Threads {
		overflow := len(t.Messages) - maxMessagesInMemory
		if overflow <= 0 {
			continue
		}
		if !s.appendArchiveLocked(t.ID, t.Messages[:overflow]) {
			continue // 归档失败:本会话不裁剪,原样保留,下次保存重试。
		}
		appended = true
		// 裁剪到最近 N 条。拷进新切片,让旧底层数组(含已归档消息)可被 GC,
		// "内存只持有最近 N 条"才名副其实。
		trimmed := make([]StoredMessage, maxMessagesInMemory)
		copy(trimmed, t.Messages[overflow:])
		t.Messages = trimmed
	}
	if appended {
		s.evictArchiveLocked()
	}
}

// evictArchiveLocked 归档目录总量守护:目录内 *.jsonl 总字节超过 archiveMaxBytes
// 时,从最旧(ModTime 最早,同刻按文件名升序保证确定性)开始整文件删除,直到
// 回到上限内。必须在 s.mu 持锁下调用(调用点 archiveOverflowLocked 已持锁)。
//
// 【为什么按整文件删而不是截断】归档行是"某线程最旧溢出消息"的顺序留档,截断
// 头部会留下半行/缺头的文件,人工合并时无法按行信任;整文件删除的粒度是
// "一个线程的全部溢出归档",语义清晰可解释。被删的是最旧线程的溢出历史
// (主文件里仍有该线程最近 maxMessagesInMemory 条),不是正在服务的消息。
//
// 【数据丢失的边界声明】这是整个归档链路里唯一会删数据的点,取舍与读路径
// 缺失(无任何代码读归档)一致:留档无限增长 → 磁盘满 → 连累主文件落库,
// 代价远大于"最旧溢出历史按上限滚动"。删除逐条 WARNING 留痕(线程 id + 释放
// 字节数),不静默。
func (s *FileStore) evictArchiveLocked() {
	limit := s.archiveMaxBytes
	if limit <= 0 {
		limit = DefaultArchiveMaxBytes
	}
	dir := filepath.Join(filepath.Dir(s.path), archiveDirName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // 目录不存在/读不了 = 无归档可清,不告警(写路径已各自告警)
	}
	type archiveFile struct {
		path string
		size int64
		mod  int64
		name string
	}
	var files []archiveFile
	var total int64
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		files = append(files, archiveFile{
			path: filepath.Join(dir, e.Name()),
			size: info.Size(),
			mod:  info.ModTime().UnixNano(),
			name: e.Name(),
		})
		total += info.Size()
	}
	if total <= limit {
		return
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].mod != files[j].mod {
			return files[i].mod < files[j].mod
		}
		return files[i].name < files[j].name
	})
	for _, f := range files {
		if total <= limit {
			return
		}
		if rerr := os.Remove(f.path); rerr != nil {
			log.Printf("[store] WARNING: 归档总量超限,删除最旧归档失败(跳过继续) file=%s: %v", f.path, rerr)
			continue
		}
		total -= f.size
		log.Printf("[store] WARNING: threads.archive 总量超上限(%d 字节),已删除最旧线程归档 %s(释放 %d 字节;该线程主文件内最近 %d 条消息不受影响)",
			limit, f.name, f.size, maxMessagesInMemory)
	}
}

// appendArchiveLocked 把 msgs 逐行追加写入 <数据目录>/threads.archive/<threadID>.jsonl,
// 并在返回前 Sync 落盘(归档是裁剪后溢出条目的唯一副本,必须先于主文件的
// "全量重写"持久化,否则"归档成功+主文件写坏"的组合可能两头皆空)。
// 每行一个 StoredMessage 的 JSON,与主文件 messages 数组元素结构完全一致。
// 返回 false 表示写失败(已打日志),调用方不得据此裁剪内存。
func (s *FileStore) appendArchiveLocked(threadID string, msgs []StoredMessage) bool {
	dir := filepath.Join(filepath.Dir(s.path), archiveDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		log.Printf("[store] WARNING: 归档目录创建失败,本次跳过归档(不影响保存,消息保留在主文件): %v", err)
		return false
	}
	f, err := os.OpenFile(filepath.Join(dir, threadID+".jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		log.Printf("[store] WARNING: 归档文件打开失败,本次跳过归档(不影响保存,消息保留在主文件) thread=%s: %v", threadID, err)
		return false
	}
	defer f.Close()
	for i := range msgs {
		line, err := json.Marshal(msgs[i])
		if err != nil {
			log.Printf("[store] WARNING: 归档条目序列化失败,本次跳过归档(不影响保存,消息保留在主文件) thread=%s: %v", threadID, err)
			return false
		}
		if _, err := f.Write(append(line, '\n')); err != nil {
			log.Printf("[store] WARNING: 归档写入失败,本次跳过归档(不影响保存,消息保留在主文件) thread=%s: %v", threadID, err)
			return false
		}
	}
	if err := f.Sync(); err != nil {
		log.Printf("[store] WARNING: 归档落盘失败,本次跳过归档(不影响保存,消息保留在主文件) thread=%s: %v", threadID, err)
		return false
	}
	return true
}
