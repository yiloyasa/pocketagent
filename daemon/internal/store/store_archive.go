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
)

// archiveDirName 是旁车归档目录名,位于 threads.json 所在的数据目录下。
const archiveDirName = "threads.archive"

// archiveOverflowLocked 把每个超过 maxMessagesInMemory 条消息的会话的最旧溢出
// 条目追加进旁车归档,成功后把内存裁剪到最近 N 条。必须在 s.mu 持锁下调用
// (saveLocked 的全部调用方均持锁),归档写入因此天然串行,无需文件级锁。
// 逐会话独立处理:某个会话归档失败只跳过该会话,其余照常。
func (s *FileStore) archiveOverflowLocked() {
	for _, t := range s.data.Threads {
		overflow := len(t.Messages) - maxMessagesInMemory
		if overflow <= 0 {
			continue
		}
		if !s.appendArchiveLocked(t.ID, t.Messages[:overflow]) {
			continue // 归档失败:本会话不裁剪,原样保留,下次保存重试。
		}
		// 裁剪到最近 N 条。拷进新切片,让旧底层数组(含已归档消息)可被 GC,
		// "内存只持有最近 N 条"才名副其实。
		trimmed := make([]StoredMessage, maxMessagesInMemory)
		copy(trimmed, t.Messages[overflow:])
		t.Messages = trimmed
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
