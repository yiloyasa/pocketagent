package agent

// 原生会话文件的删除闸门(pi / codex 共用同一份真值)。
//
// 【为什么抽成共享闸门而不是各写一份】pi 与 codex 做的是同一件不可逆的事:
// 把用户电脑上的会话文件删掉。两边的会话文件格式不同,但"敢不敢删"的前置条件
// 是同一个。历史上两家的守卫强度不对等(pi 四道、codex 只剩文件名后缀一道),
// 而"某家忘了加闸"正是各抄一份顺序的必然结果 —— 所以闸门必须是一个函数,
// 不是每家各留一段。
//
// 【四道闸,顺序固定】① 解析(EvalSymlinks 解开符号链接/junction,一切判定都在
// 解析后的路径上做)→ ② 包含性(解析后必须真在该 agent 自己的 sessions 目录**之内**)
// → ③ 扩展名 .jsonl 白名单 → ④ 身份(这个文件确实自称是请求的那条会话)。
// 任一闸不过就不删。
//
// 【共用边界:①②③ 共用,④ 由调用方注入】①②③ 只依赖"路径",与文件格式无关。
// ④ 依赖"文件怎么自称身份":pi 的 id 在**顶层** type=="session" 行上,codex 的 id
// 在 type=="session_meta" 行的 payload 里,行类型判定与 JSON 路径都不同;codex 的
// 分叉/续写 rollout 还有"文件名带 <父>_<子>、文件头存父 id"的命名约定(见
// codexRolloutIdentityMatches)。硬凑成一个"万能取 id 函数"只会造出第二份真值
// (两个格式的知识混在一处,改一个格式的解析就得动同一个函数,还会互相牵连)。
// 故 ④ 以判定函数注入:解析各归各家,而**四道闸的顺序与"不过就不删"的策略只存在
// 于本文件一处**。
//
// 【为什么保留 deleteNativeSessionFile 这个读取器版入口】pi 侧的"身份"就是
// "文件头 id == 请求 id"这一句,不需要额外约定;它作为薄适配器把读取器包成判定,
// 语义与抽成共享闸门之前逐字一致(pi_delete_test.go 钉着文案不许漂)。codex 侧
// 因为多了分叉命名约定,直接注入判定函数。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// deleteNativeSessionFile 在删掉一个原生会话文件前,把四道闸挨个过一遍;全过才
// 真正 os.Remove。身份闸(第 ④ 道)取"文件头 id == 请求 id"这一种最简形态。
//
// root 该 agent 自己的 sessions 目录(必须由真源函数给出,如 PiSessionsDir() /
// CodexSessionsDir());path 候选文件;expectID 请求删除的会话 id;
// readHeadID 读该文件头里的会话 id(各家格式自实现,返回空串 = 头里读不到身份);
// label 只进错误文案。
//
// 【目标已不在 = 幂等成功】"找到"与"删"之间文件被别的进程删掉是常事(用户在桌面
// 端同时清理),此时用户想要的终态已达成,不该报错 —— 与 pi 侧"线程还没建过原生
// 会话"的 nil 语义、以及 codex 侧 os.Remove 的 !os.IsNotExist 一致。
func deleteNativeSessionFile(root, path, expectID, label string, readHeadID func(path string) string) error {
	return deleteNativeSessionFileWithIdentity(root, path, expectID, label, func(path, expectID string) bool {
		return readHeadID(path) == expectID
	})
}

// deleteNativeSessionFileWithIdentity 是四道闸的**唯一实现**,deleteNativeSessionFile
// 只是它的一个薄适配器。
//
// 【第 ④ 道为什么注入"判定"而不只是"取 id 的读取器】身份不是一个裸字符串,各家
// 对"我是不是这条会话"的自证方式不同:pi/codex 普通 rollout 是文件头 id 等于请求
// id;codex 的分叉/续写 rollout 文件名是 <父>_<子>、文件头却存**父** id(2026-10-17
// 实测本机 16 份 rollout 里 10 份是这个形状),此时"头 id == 请求 id"会把用户正在
// 请求删除的那份文件判成"不是它"。判定归各家,顺序与"不过就不删"仍归本文件 ——
// 这正是把 ④ 做成注入点而不是各写一整条闸的理由。
//
// identityOK 只在 expectID 非空时被调用(空请求 id 一律拒删,见下);返回 false 一律
// 拒删。调用方实现里**仍必须**做到"读不到就返回 false",闸门不做兜底推断。
func deleteNativeSessionFileWithIdentity(root, path, expectID, label string, identityOK func(path, expectID string) bool) error {
	// ① 解析:符号链接/junction 一律解到实体再判,否则"在目录内"只是看起来成立。
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	// ② 包含性:rel 必须是 root 下的**真子路径**。"."(就是 sessions 目录本身)、
	// ".." 与 "../" 前缀(穿越回上一级)、以及 IsAbs 的绝对路径都拒。
	// filepath.Rel 在 Windows 上按不区分大小写比较,盘符/目录大小写差异不会被
	// 误判成越界。
	rel, err := filepath.Rel(resolvedRoot, resolved)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("%s 会话路径不在 sessions 目录中", label)
	}
	// ③ 扩展名白名单:删的是用户文件,不可逆 —— 先确认它长得像会话文件,别把同
	// 目录下的日志/临时文件当成会话删掉。
	if !strings.EqualFold(filepath.Ext(resolved), ".jsonl") {
		return fmt.Errorf("不是 %s 会话文件", label)
	}
	// ④ 身份回读:文件名会撞、目录结构能骗,**能自证"这就是那条会话"的证据只能来自
	// 文件自己**(文件名里自称的 id + 文件头里写下的 id),判定细则各家注入。
	// 请求 id 为空、或注入的判定不认 —— 一律拒删:无法证明 = 不删(fail closed)。
	if expectID == "" || !identityOK(resolved, expectID) {
		return fmt.Errorf("%s 会话 ID 与文件头不一致，未删除", label)
	}
	if err := os.Remove(resolved); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
