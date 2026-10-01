// zcode_session.go —— ZCode 的原生会话适配器(原生 session 收编 P3,设计稿 §4-P3)。
//
// 【它是什么、不是什么】runtime.NativeSessionAdapter 五方法在 zcode 家的**接线器**:
// 只做"取引用 → 调既有 session/resume·session/create 路径 → 写回引用"的胶水。
// 重挂载(session/resume + 快照记账 + 同代缓存登记)与新建(session/create + 解析
// + 写回)的唯一实现就是下方 resumeZcodeSession/createZcodeSession —— 它们是从
// zcode.go ensureSession 的重挂载/新建分支**原样拆出**的(zcode.go 的 ensureSession
// 改为调用它们,函数体逐行未改),Run 链与适配器由此共用同一份协议实现。这是
// 设计稿 D1 点名的行为漂移防线:协议一升级,握手只有一处会变。
//
// 【收编面为何收窄】设计稿 §4-P3 点名:ensureSession 的返回值快照供图片能力门控
// 消费、subscribe 订阅紧随重挂载 —— 这两件是 Run 轮次内状态机的事,**留在 Run**。
// 适配器只包"重挂载/新建 + 写回引用",不 subscribe、不消费快照原文,否则轮次内
// 状态机被适配器半接管,违反 D1。
//
// 【五方法与 zcode 现状的映射】(设计稿 §2 收编矩阵 ZCode 列)
//   - Open:调 createZcodeSession(session/create)——登记 threadMap 同代缓存 +
//     写回 ExternalSession.SessionID/Cwd;失败时在写回之前返回,旧引用不被毁;
//   - Resume:有 SessionID 时调 resumeZcodeSession(session/resume,快照记账 +
//     同代缓存登记);无引用返回 runtime.ErrNoNativeSession(对齐 memSession/
//     piSession/dshSession 契约先例);失败错误**原样上抛** —— ensureSession 的
//     "失败改开新会话"降级与账号同步错误直通都留在 Run 链(D4),适配器不吞、
//     不擅自新建;
//   - Checkpoint:无额外态(引用已写回 thread,落库归 server 层);
//   - Close:无句柄可释。zcode 的 session/close 是"真删除"语义(控制面探针实证:
//     close 后 resume 报 -32004),归删除路径 DeleteNativeSession,绝不进本方法;
//   - Source:引用 zcodeShape.sessionModel 同一表达式(D2:与 Descriptor 申报无
//     第二独立来源,source-mismatch 在结构上不可能)。
//
// 【为什么不复刻 ensureSession 的同代短路】与 dsh_session.go 同理:threadMap 同代
// 命中短路是 ensureSession(Run 链)的会话解析决策;适配器是"按落库引用重挂载"的
// 恢复入口,其语义就是重挂载 —— 重启后子进程代必新,同代短路在恢复场景无从发生。
// 若未来 Run 的会话解析改走适配器(设计稿 §6.1 收尾项),再把短路决策一并收进去。
//
// 【过渡期双路径】本轮 Run 的既有链路(zcode.go Run → ensureSession)原样不动,
// 本适配器是与之**并行**的映射层,当前唯一消费方是 openSharedRuntime 的 D5 工厂
// 注入(契约交付)。
//
// 【并发纪律】适配器方法目前只会被轮次 goroutine 调用(设计稿 §6.2):与 Run
// 现状一致,withClientCtx(有界排队)保证子进程就绪,p.client 的读取窗口与
// ensureSession 相同;threadMap 读写全走 setZcodeThreadRef/zcodeThreadRefFor
// 的专属锁(P1-2)。若未来诊断页直调 Resume,须先明确各 provider 锁纪律。
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/zcodeapp"
)

// zcodeSessionClient 是原生会话适配器(及其复用的重挂载/新建助手)对
// zcodeapp.Client 的窄接口(单测打桩,不依赖子进程;zcodeapp.Client 的方法集
// 天然满足,生产零适配 —— pi 的 piMountClient 同款手法,014db16 先例)。抽它出来
// 是为了让适配器复用既有协议路径时可用假客户端钉接线形状,而不是在适配器里
// 第二实现握手。
type zcodeSessionClient interface {
	CallCtx(ctx context.Context, method string, params interface{}) (json.RawMessage, error)
}

// 生产零适配的编译期保证:真客户端的方法集必须天然满足窄接口。
var _ zcodeSessionClient = (*zcodeapp.Client)(nil)

// resumeZcodeSession 是 session/resume 重挂载的唯一实现(zcode.go ensureSession
// 的重挂载分支原样拆出,Run 链与原生会话适配器共用):resume + 快照记账 +
// 同代缓存登记。返回快照原文供调用方消费(Run 的图片能力门控);失败原样上抛
// —— ensureSession 保留"失败改开新会话"的降级决策与账号同步错误直通,适配器
// 按 D4 直接报错,两条调用方各取所需。
func (p *ZcodeProvider) resumeZcodeSession(ctx context.Context, c zcodeSessionClient, threadID, sid string, gen int) (json.RawMessage, error) {
	// 协同 MCP 随重挂载带上(withCollabMCP,collab_inject.go):协议 schema
	// 实证 session/resume 与 session/create 同收 mcpServers;未接线时不带该键,
	// 与改造前零差异。
	raw, err := c.CallCtx(ctx, "session/resume", withCollabMCP(map[string]interface{}{"sessionId": sid}))
	if err != nil {
		return nil, err
	}
	log.Printf("[zcode] 已重挂载会话 %s", sid)
	p.recordProjection(threadID, raw)
	p.setZcodeThreadRef(threadID, threadRef{id: sid, gen: gen})
	return raw, nil
}

// createZcodeSession 是 session/create 新建的唯一实现(zcode.go ensureSession 的
// 新建分支原样拆出,Run 链与原生会话适配器共用):create + 解析 sessionId +
// 快照记账 + 同代缓存登记 + 引用写回。引用写回发生在全部成功之后,失败时
// thread 上的旧引用原样保留(落库引用归调用方掌管)。workspace 由调用方给出
// (Run 链传 req.Workspace,适配器传 th.Workspace),与写回的 Cwd 同源。
func (p *ZcodeProvider) createZcodeSession(ctx context.Context, c zcodeSessionClient, th *store.Thread, workspace string, gen int) (string, bool, json.RawMessage, error) {
	raw, err := c.CallCtx(ctx, "session/create", withCollabMCP(map[string]interface{}{
		"workspace": zcodeWorkspace(workspace),
		"mode":      "build", // 红线:显式 build
	}))
	if err != nil {
		return "", false, nil, fmt.Errorf("session/create: %w", err)
	}
	var cr struct {
		Session struct {
			SessionID string `json:"sessionId"`
		} `json:"session"`
	}
	if json.Unmarshal(raw, &cr) != nil || cr.Session.SessionID == "" {
		return "", false, nil, fmt.Errorf("session/create 应答无法解析: %s", clip(string(raw), 300))
	}
	sid := cr.Session.SessionID
	p.recordProjection(th.ID, raw)
	p.setZcodeThreadRef(th.ID, threadRef{id: sid, gen: gen})
	// server 层会在轮次后落库(Pi 同款机制);手机端据此展示外部会话引用。
	th.ExternalSession = store.ExternalSessionRef{SessionID: sid, Cwd: workspace}
	log.Printf("[zcode] 新建会话 %s(workspace=%s)", sid, workspace)
	return sid, true, raw, nil
}

// zcodeSession 是 ZCode 的原生会话适配器。无自有状态:会话引用的唯一事实在
// store.Thread.ExternalSession(createZcodeSession 写回、server 落库),适配器
// 只是 thread 到既有会话路径的那根线。
type zcodeSession struct {
	p *ZcodeProvider
}

// 编译期钉死:适配器满足统一合同;ZcodeProvider 实现可选工厂(D5)。
var (
	_ runtime.NativeSessionAdapter = (*zcodeSession)(nil)
	_ NativeSessionFactory         = (*ZcodeProvider)(nil)
)

// NativeSessionFor 实现 NativeSessionFactory(D5 工厂)。适配器无状态,每 thread
// 一个轻量壳,与 FullRuntime 每 thread 组装一次的节奏一致;当前不存在"交付不出
// 来"的运行时形态,故不返回 error(签名保留 error 是工厂合同的一部分)。
func (p *ZcodeProvider) NativeSessionFor(_ *store.Thread) (runtime.NativeSessionAdapter, error) {
	return &zcodeSession{p: p}, nil
}

// Source 声明会话事实来源。直接引用 zcodeShape.sessionModel —— 与 Descriptor()
// 的申报取同一个表达式,不写字面量:D2 让"声明"与"Source()"没有第二个独立
// 来源,VerifyRuntime(source-mismatch 检查)因此结构性通过。
func (s *zcodeSession) Source() runtime.SessionSource { return zcodeShape.sessionModel }

// clientFor 取适配器用的协议客户端:生产走 withClientCtx(有界排队,子进程没起
// 就拉起、活着就直接过,与 Run 步骤 0 同一个入口,不在适配器里第二实现就绪
// 逻辑);测试经 sessionSource 钩子注入假客户端,不拉真子进程(pi 的 sessionFor
// 同模式)。gen 必须在就绪**之后**读取:withClientCtx 可能顺带完成"杀旧起新"
// 重启,gen 只在确定"当前代"后才有效(与 ensureSession 同理,dshSession 同款纪律)。
func (s *zcodeSession) clientFor(ctx context.Context) (zcodeSessionClient, error) {
	if hook := s.p.sessionSource; hook != nil {
		return hook(ctx)
	}
	if err := s.p.withClientCtx(ctx, zcodeInitBudget, func(context.Context, *zcodeapp.Client) error { return nil }); err != nil {
		return nil, err
	}
	return s.p.client, nil
}

// Open 为 thread 建立新的原生会话(session/create 既有路径)。
//
// 【为什么不需要 pi 式的"先置空引用"】zcode 的 createZcodeSession 无条件新建
// (不按既有引用分流),失败时在写回之前返回 —— thread 上的旧引用原样保留。
// 【为什么不 subscribe】订阅与快照消费是 Run 轮次内状态机的事(设计稿 §4-P3),
// 适配器半接管就违反 D1;契约交付只需要"会话建立 + 引用写回"。
func (s *zcodeSession) Open(ctx context.Context, th *store.Thread) error {
	if th == nil {
		return fmt.Errorf("zcode: thread 为空,无法建立原生会话")
	}
	c, err := s.clientFor(ctx)
	if err != nil {
		return err
	}
	_, _, _, err = s.p.createZcodeSession(ctx, c, th, th.Workspace, int(s.p.gen.Load()))
	return err
}

// Resume 按 thread 里落库的引用重挂载已有原生会话。
//
// 【无引用必须报错】对齐 memSession/piSession/dshSession 的契约先例:没有引用
// 返回 runtime.ErrNoNativeSession 包装错,绝不假装成功、也绝不擅自新建 ——
// "失败改开新会话"是会话级产品决策(zcode 的降级条件在 ensureSession 的 Run 链),
// 留给调用方(D4)。契约套件的 stranger 反证钉死这一语义。
// 【失败原样上抛】含 AccountSyncError(账号同步错不是"会话没了",直通语义与
// ensureSession 同源)—— 适配器不吞错、不降级,谁调用谁决策。
func (s *zcodeSession) Resume(ctx context.Context, th *store.Thread) error {
	if th == nil {
		return fmt.Errorf("zcode: thread 为空,无法重挂载原生会话")
	}
	if th.ExternalSession.SessionID == "" {
		return fmt.Errorf("%w: thread=%s", runtime.ErrNoNativeSession, th.ID)
	}
	c, err := s.clientFor(ctx)
	if err != nil {
		return err
	}
	_, err = s.p.resumeZcodeSession(ctx, c, th.ID, th.ExternalSession.SessionID, int(s.p.gen.Load()))
	return err
}

// Checkpoint 把"恢复所需的引用"写回 thread。zcode 的引用(SessionID+Cwd)在
// createZcodeSession 成功那一刻已写在 thread.ExternalSession 上,持久化归 server
// 层落库 —— 这里没有额外状态可落(设计稿收编矩阵:ZCode 的 Checkpoint = 无额外态)。
func (s *zcodeSession) Checkpoint(_ context.Context, _ *store.Thread) error { return nil }

// Close 释放原生会话资源(**不删除会话内容**)。zcode 无独立的"释放句柄"调用:
// 同代失效靠 gen 代际;session/close 是真删除语义(探针实证 close 后 resume 报
// -32004),归删除路径(DeleteNativeSession),绝不能从这里发 —— 释放句柄和
// 删除内容在 zcode 是两件事。
func (s *zcodeSession) Close(_ context.Context, _ *store.Thread) error { return nil }
