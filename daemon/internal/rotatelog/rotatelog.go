// Package rotatelog 提供按尺寸轮转的单文件日志 Writer(daemon.log 的进程内轮转)。
//
// 【为什么要有它】生产链路里 paguard 把 daemon 的 stderr 重定向到 daemon.log
// (O_APPEND),而 paguard 只轮转自己的 watchdog.log —— 分发机器上 daemon.log
// 无限增长,磁盘写满会连带打断 store 落库(threads.json 全量重写失败)。轮转
// 必须做在 daemon 进程内:daemon 是长驻进程,paguard 只在崩溃/开机时拉起它,
// "拉起前轮转"管不住单次运行内的增长。
//
// 【Windows 句柄语义(已核实,方案取舍的根据)】
//   - paguard 打开 daemon.log 用的是 Go os.OpenFile(share mode 含
//     FILE_SHARE_DELETE),该句柄被子进程(daemon)继承;因此 daemon 进程内
//     rename 该文件会成功,旧句柄继续指向改名后的文件对象。
//   - 轮转后本 Writer 自己重新打开新 daemon.log 续写;daemon 的 inherited
//     stderr(2 号句柄)在 log.SetOutput 切换后不再被写 —— 没有任何写入方
//     会"写向已改名文件"留下尾大不掉的增量。(codex/pi 等子进程 stderr 由
//     stderrdrain 逐行收割后经 log 包落盘,同样走本 Writer。)
//   - 轮转用"先 Close 自己的句柄 → rename(Windows 替换语义直接覆盖 .1)→
//     重新 Open",与 paguard rotateGuardLog 同一代数(只留 .1)。
//
// 【失败降级】rename 失败(权限/杀软占用等)不丢弃日志:继续写当前文件
// (退化为"这代暂时不轮转"),并向 stderr 打一条 WARNING —— 不经 log 包,
// 避免递归(本 Writer 就是 log 的 output)。
package rotatelog

import (
	"os"
	"sync"
)

// DefaultMaxBytes 单文件轮转阈值:20MB。留两代(daemon.log + daemon.log.1)
// 磁盘占用上界 40MB,对"看门狗排障要翻日志"的场景足够,又远低于"写满磁盘"。
const DefaultMaxBytes = 20 << 20

// Writer 按尺寸轮转的追加写。Write 并发安全(log 包对 Output 串行化,但仍自持
// 锁以防其他调用方直接并发使用)。
type Writer struct {
	mu       sync.Mutex
	path     string // 当前日志文件绝对/相对路径
	maxBytes int64  // 轮转阈值;<=0 视为 DefaultMaxBytes
	f        *os.File
	size     int64 // 当前文件字节数(打开时 Stat,之后按写入累加)
	// warned 记录"rename 失败已告警过":同类故障刷屏没有信息量,一次就够。
	warned bool
}

// New 打开(或创建)path 并开始追加写。maxBytes <= 0 用 DefaultMaxBytes。
func New(path string, maxBytes int64) (*Writer, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	w := &Writer{path: path, maxBytes: maxBytes, f: f}
	if st, serr := f.Stat(); serr == nil {
		w.size = st.Size()
	}
	return w, nil
}

// Write 实现 io.Writer。超阈值先轮转(留一代 .1)再写。
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	limit := w.maxBytes
	if limit <= 0 {
		limit = DefaultMaxBytes
	}
	// 与 paguard/eventstore 同口径:再写这批就要超限 → 先轮转再写。
	if w.size > 0 && w.size+int64(len(p)) > limit {
		w.rotate()
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

// Close 关闭当前句柄(进程退出/测试收尾)。
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}

// rotate 留一代再续写:Close → rename(path, path+".1",替换旧 .1)→ 重开。
// 调用方已持锁。任一步失败:保持当前句柄可用(不轮转),stderr 告警一次。
func (w *Writer) rotate() {
	f := w.f
	if f != nil {
		_ = f.Close()
		w.f = nil
	}
	if err := os.Rename(w.path, w.path+".1"); err != nil {
		// rename 失败:退回继续追加写原文件(这代暂时不轮转)。句柄已关需重开;
		// 重开也失败则本 Writer 永久失效(调用方 log 会把错误写 stderr,不会
		// 经由本 Writer 递归)。
		nf, oerr := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if oerr != nil {
			if !w.warned {
				w.warned = true
				os.Stderr.WriteString("[rotatelog] WARNING: daemon.log 轮转失败且原文件重开失败,本进程日志停写: " + oerr.Error() + "\n")
			}
			return
		}
		w.f = nf
		if !w.warned {
			w.warned = true
			os.Stderr.WriteString("[rotatelog] WARNING: daemon.log 轮转(rename)失败,继续写当前文件: " + err.Error() + "\n")
		}
		return
	}
	nf, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		if !w.warned {
			w.warned = true
			os.Stderr.WriteString("[rotatelog] WARNING: daemon.log 轮转后新文件打开失败,本进程日志停写: " + err.Error() + "\n")
		}
		return
	}
	w.f = nf
	w.size = 0
	w.warned = false // 恢复正常后允许报告下一次新故障
}
