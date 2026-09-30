// dsh_session.go —— DSH 的原生会话适配器(原生 session 收编 P2,设计稿 §4-P2)。
//
// 【它是什么、不是什么】runtime.NativeSessionAdapter 五方法在 dsh 家的**接线器**:
// 只做"取引用 → 调 dsh.go 既有 remountSession/createSession → 写回引用"的胶水。
// 能力门控(sessionCapabilities.resume/list)、session/list 双因子校验(sessionId+
// cwd)、session/resume 握手与降级位(downgradeable)全部复用 remountSession 这
// 唯一实现 —— 适配器里没有任何第二实现。这是设计稿 D1 点名的行为漂移防线:
// rc.3 的 cwd 必填、list 校验等实测纪律只有一处会变。
//
// 【五方法与 dsh 现状的映射】(设计稿 §2 收编矩阵 DSH 列)
//   - Open:调 createSession(session/new)——登记 threadMap 同代缓存 + 写回
//     ExternalSession.SessionID/Cwd + 灌"新会话初始值";失败时引用原样保留
//     (createSession 在写回之前返回,旧引用不被毁);
//   - Resume:有 SessionID 时调 remountSession(能力门控 + list 校验 + resume);
//     无引用返回 runtime.ErrNoNativeSession(对齐 memSession/piSession 契约先例,
//     backend.go:995 语义);失败错误**原样上抛**,保留 *dshRemountError 的
//     downgradeable 位供调用方决策(D4)——"数据错位(list 不在列)硬拒绝"与
//     "升级断裂(在列 + resume -32602)可降级重建"两类失败由此可区分,但重建
//     (rebuildSessionAfterUpgrade)留在 Run 的降级链里,适配器不吞(设计稿
//     §4-P2:"升级重建路径留在 Run,适配器不吞");
//   - Checkpoint:无额外态(引用已写回 thread,落库归 server 层);
//   - Close:无句柄可释。dsh 的 session/close 是"静默取消+排空+落盘+销毁"的
//     **删除**语义(dshapp.SessionClose),不是句柄释放 —— 归删除路径
//     DeleteDshSession,不进本方法;
//   - Source:引用 dshShape.sessionModel 同一表达式(D2:与 Descriptor 申报无
//     第二独立来源,source-mismatch 在结构上不可能)。
//
// 【为什么不复刻 ensureSession 的同代短路】threadMap 同代命中短路是 ensureSession
// (Run 链)的会话解析决策;适配器是"按落库引用重挂载"的恢复入口,其语义就是
// 重挂载 —— 重启后子进程代必新,同代短路在恢复场景无从发生。若未来 Run 的会话
// 解析改走适配器(设计稿 §6.1 收尾项),再把短路决策一并收进去,而不是现在在
// 两处各写一份。
//
// 【过渡期双路径】本轮 Run 的既有链路(dsh.go Run → ensureSession)原样不动,
// 本适配器是与之**并行**的映射层,当前唯一消费方是 openSharedRuntime 的 D5 工厂
// 注入(契约交付)。
//
// 【并发纪律】适配器方法目前只会被轮次 goroutine 调用(设计稿 §6.2):与 Run
// 现状一致,withClientCtx(有界排队)保证子进程就绪,p.client 的读取窗口与
// ensureSession 相同;若未来诊断页直调 Resume,须先明确各 provider 锁纪律。
package agent

import (
	"context"
	"fmt"

	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// dshSession 是 DSH 的原生会话适配器。无自有状态:会话引用的唯一事实在
// store.Thread.ExternalSession(createSession 写回、server 落库),适配器只是
// thread 到 dsh.go 既有会话路径的那根线。
type dshSession struct {
	p *DshProvider
}

// 编译期钉死:适配器满足统一合同;DshProvider 实现可选工厂(D5)。
var (
	_ runtime.NativeSessionAdapter = (*dshSession)(nil)
	_ NativeSessionFactory         = (*DshProvider)(nil)
)

// NativeSessionFor 实现 NativeSessionFactory(D5 工厂)。适配器无状态,每 thread
// 一个轻量壳,与 FullRuntime 每 thread 组装一次的节奏一致;当前不存在"交付不出
// 来"的运行时形态,故不返回 error(签名保留 error 是工厂合同的一部分,P3 的
// 家可能需要)。
func (p *DshProvider) NativeSessionFor(_ *store.Thread) (runtime.NativeSessionAdapter, error) {
	return &dshSession{p: p}, nil
}

// Source 声明会话事实来源。直接引用 dshShape.sessionModel —— 与 Descriptor()
// 的申报取同一个表达式,不写字面量:D2 让"声明"与"Source()"没有第二个独立
// 来源,VerifyRuntime(source-mismatch 检查)因此结构性通过。
func (s *dshSession) Source() runtime.SessionSource { return dshShape.sessionModel }

// ensureClient 保证子进程活着且已完成 initialize 握手(懒启动 + 能力快照),
// 与 Run 步骤 0 同一个入口(withClientCtx),不在适配器里第二实现就绪逻辑。
// remountSession 的能力门控读 p.caps、SessionList/Resume 走 p.client,都以此为
// 前置;createSession 直用 p.client 同理。
func (s *dshSession) ensureClient(ctx context.Context) error {
	return s.p.withClientCtx(ctx, dshInitBudget, func(context.Context, dshClient) error { return nil })
}

// Open 为 thread 建立新的原生会话(session/new 既有路径)。
//
// 【为什么不需要 pi 式的"先置空引用"】dsh 的 createSession 无条件新建(不按
// 既有引用分流),失败时在写回之前返回 —— thread 上的旧引用原样保留,落库引用
// 归调用方掌管。成功时 ExternalSession.SessionID/Cwd 被新会话覆盖,即"Open 的
// 语义是新建"。
func (s *dshSession) Open(ctx context.Context, th *store.Thread) error {
	if th == nil {
		return fmt.Errorf("dsh: thread 为空,无法建立原生会话")
	}
	if err := s.ensureClient(ctx); err != nil {
		return err
	}
	// gen 在就绪之后读取(与 ensureSession 一致:withClient 可能重启子进程,
	// gen 只在确定"当前代"后才有效),createSession 据此登记同代缓存。
	_, err := s.p.createSession(ctx, th, th.Workspace, int(s.p.gen.Load()))
	return err
}

// Resume 按 thread 里落库的引用重挂载已有原生会话。
//
// 【无引用必须报错】对齐 memSession/piSession 的契约先例:没有引用返回
// runtime.ErrNoNativeSession 包装错,绝不假装成功、也绝不擅自新建 —— "失败改
// 开新会话"是会话级产品决策,留给调用方(D4)。契约套件的 stranger 反证
// (contract_suite.go SessionResumeAfterRestart)钉死这一语义。
// 【失败类型区分(D4)】remountSession 返回 *dshRemountError,downgradeable 位
// 区分"数据错位(list 不在列/能力缺席)→ 硬拒绝"与"升级断裂(在列 + -32602)
// → 可降级重建";适配器把错误**原样上抛**,调用方用 errors.As 取位自行决策 ——
// 一次性重建(rebuildSessionAfterUpgrade)只属于用户主动发消息的 Run 降级链,
// 适配器不吞这条决策。
func (s *dshSession) Resume(ctx context.Context, th *store.Thread) error {
	if th == nil {
		return fmt.Errorf("dsh: thread 为空,无法重挂载原生会话")
	}
	ext := th.ExternalSession
	if ext.SessionID == "" {
		return fmt.Errorf("%w: thread=%s", runtime.ErrNoNativeSession, th.ID)
	}
	if err := s.ensureClient(ctx); err != nil {
		return err
	}
	gen := int(s.p.gen.Load())
	sid, catOpts, err := s.p.remountSession(ctx, ext)
	if err != nil {
		return err // 原样上抛:dshRemountError 的降级位留给调用方(D4)
	}
	// 与 ensureSession 重挂载成功路径相同的登记语义:同代缓存 + 目录刷新
	// (探针修正②:resume 应答带 configOptions,桌面端目录可能在停机期间变了)。
	s.p.setDshThreadRef(th.ID, threadRef{id: sid, gen: gen})
	s.p.refreshCatalogFromResume(catOpts)
	return nil
}

// Checkpoint 把"恢复所需的引用"写回 thread。dsh 的引用(SessionID+Cwd)在
// createSession 成功那一刻已写在 thread.ExternalSession 上,持久化归 server 层
// 落库 —— 这里没有额外状态可落(设计稿收编矩阵:dsh 的 Checkpoint = 无额外态)。
func (s *dshSession) Checkpoint(_ context.Context, _ *store.Thread) error { return nil }

// Close 释放原生会话资源(**不删除会话内容**)。dsh 无独立的"释放句柄"调用:
// 同代失效靠 gen 代际;session/close 是销毁语义,归删除路径(DeleteDshSession),
// 绝不能从这里发 —— 释放句柄和删除内容在 dsh 是两件事。
func (s *dshSession) Close(_ context.Context, _ *store.Thread) error { return nil }

// DeleteDshSession 远端删除:session/close(静默取消+排空+落盘+销毁)。
//
// 【能力门控】对齐 remountSession 的纪律(§5.2:一切扩展方法的使用以 initialize
// 应答为准,不写死协议假设):sessionCapabilities.close 不在场 → 显式报错且不
// 发调用,不猜。close=true 已由 P0 探针实测(docs/verify/evidence/dsh-p0-probe.md,
// 探针第 1 项握手快照 list/resume/close 在场)。
// 【失败语义】失败返回明确错误,由 server 层记日志但不让本地删除失败
// (zcode/codex 同款口径;墓碑才是防复活的权威)。
func (p *DshProvider) DeleteDshSession(sessionID string) error {
	return p.withClient(func(ctx context.Context, c dshClient) error {
		if p.caps != nil && !p.caps.CanClose() {
			return fmt.Errorf("dsh %s 不支持会话关闭(sessionCapabilities.close 不在场),无法删除会话 %s",
				p.caps.AgentInfo.Version, sessionID)
		}
		return c.SessionClose(ctx, sessionID)
	})
}
