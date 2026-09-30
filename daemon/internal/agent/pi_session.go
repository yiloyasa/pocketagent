// pi_session.go —— Pi 的原生会话适配器(原生 session 收编 P1,设计稿 §4-P1)。
//
// 【它是什么、不是什么】runtime.NativeSessionAdapter 五方法在 Pi 家的**接线器**:
// 只做"取引用 → 调 pi.go 既有 mount → 写回引用"的胶水。握手纪律(同代短路 /
// switch_session + get_state 校验)与超时预算(piSwitchBudget/piCallBudget)全部
// 复用 mount 这唯一实现 —— 适配器里没有任何第二实现。这是设计稿 D1 点名的
// 行为漂移防线:协议一升级,握手只有一处会变。
//
// 【五方法与 Pi 现状的映射】(设计稿 §2 收编矩阵 Pi 列)
//   - Open:置空引用后调 mount → 走其"新会话"分支(get_state 回填
//     ExternalSession.SessionID+SessionFile,pi.go mount 新会话分支);
//   - Resume:有 SessionFile 时调 mount → 其内部已做同代短路 + switch_session
//     握手校验;无引用返回 runtime.ErrNoNativeSession(对齐 memSession.Resume
//     的契约先例,runtime/backend.go:995 语义);
//   - Checkpoint:无额外态(引用已写回 thread,落库归 server 层);
//   - Close:无句柄可释(pi 一个子进程=一个会话,switch_session 即换挂);
//   - Source:引用 piShape.sessionModel 同一表达式(D2:与 Descriptor 申报无
//     第二独立来源,source-mismatch 在结构上不可能)。
//
// 【过渡期双路径】本轮 Run 的既有链路(pi.go Run → clientFor → mount)原样不动,
// 本适配器是与之**并行**的映射层,当前唯一消费方是 openSharedRuntime 的 D5 工厂
// 注入(契约交付)。后续切换点:Run 与 ensureMounted 的挂载调用改走适配器
// (设计稿 §6.1,P3 之后的收尾项,需单独立项)。
//
// 【并发纪律】适配器方法目前只会被轮次 goroutine 调用(设计稿 §6.2):mount 内部
// 的 clients map 读写由 PiProvider.mu 保护,与 Run 现状一致;若未来诊断页直调
// Resume,须先明确各 provider 锁纪律。
package agent

import (
	"context"
	"fmt"

	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// NativeSessionFactory 是 D5 的可选工厂合同(设计稿 §3-D5):provider 实现了它,
// openSharedRuntime 才注入 rt.NativeSession;没实现的家(codex/zcode/dsh,截至
// P1)维持 nil —— 六家增量收编互不牵连。与 Steerer/PrefsAware 同款类型断言探测,
// 无注册名单(任何按 agent 名字分支的写法都违反 README §0.5)。
type NativeSessionFactory interface {
	// NativeSessionFor 为 thread 交付原生会话适配器。
	// 返回 error 表示"此刻交付不出来",调用方如实上报,不得假装交付成功。
	NativeSessionFor(th *store.Thread) (runtime.NativeSessionAdapter, error)
}

// piSession 是 Pi 的原生会话适配器。无自有状态:会话引用的唯一事实在
// store.Thread.ExternalSession(mount 写回、server 落库),适配器只是 thread 到
// mount 的那根线。
type piSession struct {
	p *PiProvider
}

// 编译期钉死:适配器满足统一合同;PiProvider 实现可选工厂(D5)。
var (
	_ runtime.NativeSessionAdapter = (*piSession)(nil)
	_ NativeSessionFactory         = (*PiProvider)(nil)
)

// NativeSessionFor 实现 NativeSessionFactory(D5 工厂)。适配器无状态,每 thread
// 一个轻量壳,与 FullRuntime 每 thread 组装一次的节奏一致;当前不存在"交付不出
// 来"的运行时形态,故不返回 error(签名保留 error 是工厂合同的一部分,P2/P3 的
// 家可能需要)。
func (p *PiProvider) NativeSessionFor(_ *store.Thread) (runtime.NativeSessionAdapter, error) {
	return &piSession{p: p}, nil
}

// Source 声明会话事实来源。直接引用 piShape.sessionModel —— 与 Descriptor()
// 的申报取同一个表达式,不写字面量:D2 让"声明"与"Source()"没有第二个独立
// 来源,VerifyRuntime(agent_runtime.go 的 source-mismatch 检查)因此结构性通过。
func (s *piSession) Source() runtime.SessionSource { return piShape.sessionModel }

// Open 为 thread 建立新的原生会话。
//
// 【为什么先置空引用】mount 以 thread.ExternalSession.SessionFile 是否为空分流
// (非空 → switch_session 重挂载;空 → get_state 回填新建)。Open 的语义是
// "新建",置空引用即复用 mount 的新会话分支,不在适配器里另写分流 —— 那会是
// 第二实现。失败时还原旧引用:落库引用归调用方掌管,失败的 Open 不得悄悄毁掉它
// (引用与子进程实际挂载态由此保持一致,后续 Resume 仍按旧引用决策)。
func (s *piSession) Open(ctx context.Context, th *store.Thread) error {
	if th == nil {
		return fmt.Errorf("pi: thread 为空,无法建立原生会话")
	}
	saved := th.ExternalSession
	th.ExternalSession = store.ExternalSessionRef{}
	c, err := s.mountFor(th)
	if err != nil {
		th.ExternalSession = saved
		return err
	}
	if err := s.p.mount(ctx, th, c); err != nil {
		th.ExternalSession = saved
		return err
	}
	return nil
}

// Resume 按 thread 里落库的引用重挂载已有原生会话。
//
// 【无引用必须报错】对齐 memSession.Resume 的契约先例:没有引用返回
// runtime.ErrNoNativeSession 包装错,绝不假装成功、也绝不擅自新建 —— "失败改
// 开新会话"是会话级产品决策,各家降级条件不同,留给调用方(D4)。契约套件的
// stranger 反证(contract_suite.go SessionResumeAfterRestart)钉死这一语义。
// 【有引用直接递给 mount】同代短路(子进程还挂着目标会话)与 switch_session +
// get_state 握手校验都在 mount 内部,这里只负责把引用送进去。
func (s *piSession) Resume(ctx context.Context, th *store.Thread) error {
	if th == nil {
		return fmt.Errorf("pi: thread 为空,无法重挂载原生会话")
	}
	if th.ExternalSession.SessionFile == "" {
		return fmt.Errorf("%w: thread=%s", runtime.ErrNoNativeSession, th.ID)
	}
	c, err := s.mountFor(th)
	if err != nil {
		return err
	}
	return s.p.mount(ctx, th, c)
}

// Checkpoint 把"恢复所需的引用"写回 thread。Pi 的引用(SessionFile+SessionID)
// 在 mount 成功那一刻已写在 thread.ExternalSession 上,持久化归 server 层落库
// —— 这里没有额外状态可落(设计稿收编矩阵:Pi 的 Checkpoint = 无额外态)。
func (s *piSession) Checkpoint(_ context.Context, _ *store.Thread) error { return nil }

// Close 释放原生会话资源(**不删除会话内容**)。Pi 无显式关闭:一个子进程就是
// 一个会话,switch_session 即换挂;子进程生命周期由 clients map 与 Run/删除路径
// (DropThread/DeletePiSession)掌管,这里无句柄可释,也不是删内容的地方。
func (s *piSession) Close(_ context.Context, _ *store.Thread) error { return nil }

// mountFor 取(或建)挂载用的子进程客户端:生产走 clientFor(复用存活子进程或
// 冷启新进程,含 GlobalHandler/守护扩展等接线);测试经 sessionFor 钩子注入假
// 客户端,不拉真子进程(与 remount 钩子同模式)。适配器不自带 timeout 第二实现
// —— 预算(piSwitchBudget/piCallBudget)由 mount 内部照旧施加。
func (s *piSession) mountFor(th *store.Thread) (piMountClient, error) {
	if hook := s.p.sessionFor; hook != nil {
		return hook(th)
	}
	return s.p.clientFor(th, th.Workspace)
}
