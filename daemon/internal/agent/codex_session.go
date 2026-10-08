// codex_session.go —— Codex 的原生会话适配器(原生 session 收编 P3,设计稿 §4-P3)。
//
// 【它是什么、不是什么】runtime.NativeSessionAdapter 五方法在 codex 家的**接线器**:
// 只做"取引用 → 调既有 thread/start·thread/resume 路径 → 写回引用"的胶水。
// resume 的 config 透传与失败语义整体复用 callThreadResume(唯一实现);新建的
// threadId 三级回退解析整体复用 startCodexThread(从 codex.go Run 的新建分支
// **原样拆出**,Run 链与适配器共用同一份协议实现)。适配器里没有任何第二实现
// —— 这是设计稿 D1 点名的行为漂移防线。
//
// 【codex 的特殊点】(设计稿 §2/§4-P3 点名"结构差异最大、收益最低")
//   - 引用在专属字段 Thread.CodexThreadID 而非通用 ExternalSession(设计稿
//     "不做清单":不统一 id 命名空间,迁移动 store 与落库数据,风险外溢);
//     因此 Resume 只认 CodexThreadID,Checkpoint 是 no-op(引用已在 thread 上);
//   - Run 链的 resume 带 config 覆盖(线程快照 > 全局),失败"改开新线程"的
//     降级决策留在 Run —— 适配器 Resume 失败原样上抛、绝不发 thread/start(D4:
//     降级顺序不变,谁调用谁决策;降级缺失的历史教训见 callThreadResume 注释);
//   - ephemeral 线程是一次性请求参数(req.Ephemeral),不属于 thread 的会话
//     生命周期,适配器面不携带。
//
// 【五方法与 codex 现状的映射】(设计稿 §2 收编矩阵 Codex 列)
//   - Open:调 startCodexThread(thread/start)——登记 threadMap 同代缓存 +
//     写回 CodexThreadID;失败时在写回之前返回,旧引用不被毁;
//   - Resume:有 CodexThreadID 时调 callThreadResume(config 透传);无引用返回
//     runtime.ErrNoNativeSession(对齐 memSession/piSession/dshSession/
//     zcodeSession 契约先例);成功后登记同代缓存(与 Run 的 resume 成功路径
//     同款);
//   - Checkpoint:无额外态(引用已写回 thread.CodexThreadID,落库归 server 层);
//   - Close:无句柄可释。codex 会话存活于子进程,threadRef 带 gen 代际失效
//     (codex.go threadRef 注释);thread/delete 是真删除,归删除路径
//     DeleteNativeSession,绝不进本方法;
//   - Source:引用 codexShape.sessionModel 同一表达式(D2:与 Descriptor 申报无
//     第二独立来源,source-mismatch 在结构上不可能)。
//
// 【P1-7:双路径已退役】Run 的会话解析(codex.go Run 装配段)不再直连
// p.client.Call 的 thread/resume·thread/start:重挂载统一经本适配器 Resume、
// 新建统一经 openForRun(两者内部复用 callThreadResume/startCodexThread 唯一
// 实现)。本适配器不再只是 openSharedRuntime D5 工厂注入的"并行映射层",而是
// codex 会话解析的单一协议入口(Run 链保留的只有同代短路、"resume 失败改开
// 新线程"降级与目标暂存补发,均属 D4 派给 Run 的决策与轮次内编排)。
//
// 【并发纪律】适配器方法目前只会被轮次 goroutine 调用(设计稿 §6.2):与 Run
// 现状一致,withClient(有界排队)保证子进程就绪,p.client 的读取窗口与 Run
// 相同;threadMap 读写全走 setCodexThreadRef/codexThreadRef。若未来诊断页直调
// Resume,须先明确各 provider 锁纪律。
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/pocketagent/daemon/internal/codexapp"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// startCodexThread 封装 thread/start 调用与 threadId 三级回退解析(codex.go Run
// 的新建分支原样拆出,Run 链与原生会话适配器共用同一段,避免两处各写一遍、
// 形状漂移;与 callThreadResume 同款手法)。
//
// 【三级回退】threadId 顶层在场是常态;thread/started 实测过 id 藏在 thread.id
// (S0 证据),裸 id 是更旧版本的形状 —— 三级都试,解析不出就报错,绝不编 id。
// call 形同 p.client.Call(method, params),用函数参数注入便于单测(无需真实
// 子进程)。cfgOverride 全程只读。
func startCodexThread(cwd string, ephemeral bool, cfgOverride map[string]interface{}, call func(method string, params interface{}) (json.RawMessage, error)) (string, error) {
	startParams := map[string]interface{}{"cwd": cwd}
	if ephemeral {
		startParams["ephemeral"] = true
	}
	if len(cfgOverride) > 0 {
		startParams["config"] = cfgOverride
	}
	// 与 turn/start 一样打日志:让"回答详略/思考摘要真的带上了"可被外部核验。
	log.Printf("[codex] thread/start cwd=%s config=%v", cwd, cfgOverride)
	raw, err := call("thread/start", startParams)
	// 【反问卡】注意:此处不做"带 features 覆盖失败→摘除重试"的护栏。
	// 该护栏是为"注入 under-development 特性"特设的逃生舱,现在已不再注入
	// (见 askUserInputSupported 注释),无 features 键可摘,删干净以免留下死分支。
	if err != nil {
		return "", fmt.Errorf("thread/start: %w", err)
	}
	var res struct {
		ThreadId string `json:"threadId"`
		Thread   struct {
			Id string `json:"id"`
		} `json:"thread"`
		Id string `json:"id"`
	}
	if json.Unmarshal(raw, &res) != nil || (res.ThreadId == "" && res.Thread.Id == "" && res.Id == "") {
		return "", fmt.Errorf("thread/start 应答无法解析: %s", clip(string(raw), 300))
	}
	tid := res.ThreadId
	if tid == "" {
		tid = res.Thread.Id
	}
	if tid == "" {
		tid = res.Id
	}
	return tid, nil
}

// codexSession 是 Codex 的原生会话适配器。无自有状态:会话引用的唯一事实在
// store.Thread.CodexThreadID(专属字段,startCodexThread 的调用方写回、server
// 落库),适配器只是 thread 到既有会话路径的那根线。
type codexSession struct {
	p *CodexProvider
}

// 编译期钉死:适配器满足统一合同;CodexProvider 实现可选工厂(D5)。
var (
	_ runtime.NativeSessionAdapter = (*codexSession)(nil)
	_ NativeSessionFactory         = (*CodexProvider)(nil)
)

// NativeSessionFor 实现 NativeSessionFactory(D5 工厂)。适配器无状态,每 thread
// 一个轻量壳,与 FullRuntime 每 thread 组装一次的节奏一致;当前不存在"交付不出
// 来"的运行时形态,故不返回 error(签名保留 error 是工厂合同的一部分)。
func (p *CodexProvider) NativeSessionFor(_ *store.Thread) (runtime.NativeSessionAdapter, error) {
	return &codexSession{p: p}, nil
}

// Source 声明会话事实来源。直接引用 codexShape.sessionModel —— 与 Descriptor()
// 的申报取同一个表达式,不写字面量:D2 让"声明"与"Source()"没有第二个独立
// 来源,VerifyRuntime(source-mismatch 检查)因此结构性通过。
func (s *codexSession) Source() runtime.SessionSource { return codexShape.sessionModel }

// callFor 取适配器用的 JSON-RPC 调用源:生产走 withClient(与 Run 步骤 0 同一个
// 入口,子进程没起就 initialize 拉起,不在适配器里第二实现就绪逻辑)后借
// p.client.Call;测试经 sessionCall 钩子注入假调用,不拉真子进程(pi 的
// sessionFor 同模式)。gen 由调用方在就绪**之后**读取:withClient 可能顺带完成
// "杀旧起新"重启,gen 只在确定"当前代"后才有效(与 ensureSession 同理)。
func (s *codexSession) callFor() (func(method string, params interface{}) (json.RawMessage, error), error) {
	if hook := s.p.sessionCall; hook != nil {
		return hook, nil
	}
	if err := s.p.withClient(func(*codexapp.Client) error { return nil }); err != nil {
		return nil, err
	}
	return s.p.client.Call, nil
}

// Open 为 thread 建立新的原生会话(thread/start 既有路径)。
//
// 【为什么不需要 pi 式的"先置空引用"】codex 的新建是无条件 thread/start(不按
// 既有引用分流),失败时在写回之前返回 —— thread 上的旧引用原样保留。
// 【为什么不 stashGoal】"新建线程后暂存目标补发"是 Run 的轮次编排动作
// (codex.go Run 新建分支),不属于会话生命周期;适配器只管"会话建立 + 引用写回"。
func (s *codexSession) Open(ctx context.Context, th *store.Thread) error {
	if th == nil {
		return fmt.Errorf("codex: thread 为空,无法建立原生会话")
	}
	_, err := s.openForRun(ctx, th, false, s.p.ConfigOverrideFor(th))
	return err
}

// openForRun Run 链(新建线程)的入口(P1-7):与 Open 共用同一 startCodexThread
// 实现(三级回退解析 + 同代缓存登记 + 引用写回的语义一字不差,这里没有第二份)。
// ephemeral 与 config 覆盖由调用方显式给出 —— 两者都是 Run 的一次性请求参数
// (req.Ephemeral / 线程快照 > 全局),不属于 thread 的会话生命周期,适配器合同
// 面 Open 不携带(见类型注释);显式传参保留旧链取值语义,Run 已算好的
// cfgOverride 不重算第二遍(dshSession.openForRun 同款纪律)。
func (s *codexSession) openForRun(ctx context.Context, th *store.Thread, ephemeral bool, cfgOverride map[string]interface{}) (string, error) {
	if th == nil {
		return "", fmt.Errorf("codex: thread 为空,无法建立原生会话")
	}
	call, err := s.callFor()
	if err != nil {
		return "", err
	}
	tid, err := startCodexThread(th.Workspace, ephemeral, cfgOverride, call)
	if err != nil {
		return "", err
	}
	s.p.setCodexThreadRef(th.ID, threadRef{id: tid, gen: int(s.p.gen.Load())})
	th.CodexThreadID = tid // server 层会在轮次后落库(与 Run 新建分支同款)
	return tid, nil
}

// Resume 按 thread 里落库的引用重挂载已有原生会话。
//
// 【无引用必须报错】codex 的引用在专属字段 CodexThreadID(设计稿"不做清单":
// 不迁 ExternalSession),为空即 runtime.ErrNoNativeSession 同族错误,绝不假装
// 成功、也绝不擅自新建 —— "失败改开新线程"是 Run 链的降级决策(历史教训:
// 缺降级护栏时表现为"AI 失忆",见 Run resume 分支注释),留给调用方(D4)。
// 【config 覆盖照旧透传】整体复用 callThreadResume:线程快照 > 全局的覆盖语义、
// 日志口径与 Run 的 resume 路径一字不差,适配器不重排任何降级顺序。
func (s *codexSession) Resume(ctx context.Context, th *store.Thread) error {
	if th == nil {
		return fmt.Errorf("codex: thread 为空,无法重挂载原生会话")
	}
	if th.CodexThreadID == "" {
		return fmt.Errorf("%w: thread=%s", runtime.ErrNoNativeSession, th.ID)
	}
	call, err := s.callFor()
	if err != nil {
		return err
	}
	if err := s.p.callThreadResume(th.CodexThreadID, s.p.ConfigOverrideFor(th), call); err != nil {
		return err // 原样上抛:降级改开新线程的决策留给调用方(D4)
	}
	s.p.setCodexThreadRef(th.ID, threadRef{id: th.CodexThreadID, gen: int(s.p.gen.Load())})
	return nil
}

// Checkpoint 把"恢复所需的引用"写回 thread。codex 的引用(CodexThreadID)在
// thread 专属字段上(startCodexThread 成功后写回,server 层轮次后落库)—— 这里
// 没有额外状态可落(设计稿收编矩阵:Codex 的 Checkpoint = 无额外态,引用在
// thread 上)。
func (s *codexSession) Checkpoint(_ context.Context, _ *store.Thread) error { return nil }

// Close 释放原生会话资源(**不删除会话内容**)。codex 无显式关闭:会话存活于
// 子进程,随子进程生命周期(threadRef 带 gen 代际失效);thread/delete 是真删除,
// 归删除路径(DeleteNativeSession),绝不能从这里发 —— 释放句柄和删除内容在
// codex 是两件事。
func (s *codexSession) Close(_ context.Context, _ *store.Thread) error { return nil }
