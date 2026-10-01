// dsh_config_options.go —— DshProvider 的原生设置目录透出(NativeConfigOptions)。
//
// 【地面真值】探针两份(docs/verify/evidence/dsh-p0-probe.md §2 +
// dsh-config-persist-probe.md):
//   - session/new|resume 应答顶层带 configOptions 数组
//     {id,name,category,type,currentValue,options};候选两种形状(扁平/分组),
//     展平见 dshapp.FlattenOptionLeaves;
//   - session/set_config_option 参数名是 configId(rc.3 zod 契约),value 原样
//     (model 的双层 JSON 编码串逐字节回传,裸模型名被原生拒绝 —— 红线);
//   - 写仅会话内生效、不持久:thread 绑定 → 当场生效;无线程 → 记"新会话初始值"。
//
// 【无静态名单】目录、候选、当前值全部来自原生应答,本文件只做透传与校验;
// 上游加配置项,手机端自动出现,daemon 不改。
package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/pocketagent/daemon/internal/dshapp"
	"github.com/pocketagent/daemon/internal/store"
)

const (
	dshCatalogBudget = 12 * time.Second // 一次性瞬态会话取样的整段预算(与 Models() 同量级)
	// dshOptionNote 原生设置行的生效范围说明(探针实证:仅会话内生效、不持久)。
	dshOptionNote = "按会话生效(dsh 原生行为):设置的值成为新会话的初始值,已有会话保持各自当前值。"
)

// 编译期钉死:DshProvider 提供原生设置目录。
var _ NativeConfigOptions = (*DshProvider)(nil)

// setRawCatalog 登记最近一次原生目录(完整 configOptions,非仅模型面)。
// 调用方须持 catMu。
func (p *DshProvider) setRawCatalog(opts []dshapp.ConfigOption) {
	p.rawOpts = append([]dshapp.ConfigOption(nil), opts...)
}

// rawCatalogSnapshot 返回原生目录快照(可能为 nil = 尚未取样)。
func (p *DshProvider) rawCatalogSnapshot() []dshapp.ConfigOption {
	p.catMu.Lock()
	defer p.catMu.Unlock()
	return p.rawOpts
}

// nativeDefaultsSnapshot 返回"新会话初始值"快照(线程安全)。
func (p *DshProvider) nativeDefaultsSnapshot() map[string]string {
	p.catMu.Lock()
	defer p.catMu.Unlock()
	out := make(map[string]string, len(p.nativeDefaults))
	for k, v := range p.nativeDefaults {
		out[k] = v
	}
	return out
}

// ConfigOptionsCatalog 实现 agent.NativeConfigOptions。
//
// t == nil:"新会话初始值"视角 —— 一次性瞬态会话取样(与 fetchModels 同款纪律:
// 用系统临时目录作业、用完即毁),并把已存的初始值灌进去,保证 currentValue
// 反映设置页当前生效的语义。
// t != nil:该会话的实时目录(session/resume 应答带 configOptions,探针修正②);
// 会话已在本桥内活跃时 resume 被原生拒绝(already active),回最近一次已知目录
// 快照兜底(见函数体内判别);轮次执行中拒绝(与共享子进程的纪律一致:不打断
// 正在跑的流)。
func (p *DshProvider) ConfigOptionsCatalog(ctx context.Context, t *store.Thread) ([]NativeConfigOption, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if t == nil {
		opts, err := p.sampleCatalogOptions(ctx)
		if err != nil {
			return nil, err
		}
		return dshOptionsToRows(opts), nil
	}
	if p.turnActive.Load() {
		return nil, ErrBusy
	}
	sid := p.dshSessionIDFor(t)
	if sid == "" {
		return nil, errors.New("该会话尚未建立 dsh 原生会话(首轮对话后可查看实时原生设置)")
	}
	var opts []dshapp.ConfigOption
	err := p.withClientCtx(ctx, dshInitBudget, func(ctx context.Context, c dshClient) error {
		// cwd 按 rc.3 契约带上(缺失时 resume 被 -32602 拒;落库没记 cwd 的老
		// 线程传空串,由 resume 走原参形态并如实失败,不在设置页路径上猜目录)。
		st, rerr := c.SessionResume(ctx, sid, t.ExternalSession.Cwd)
		if rerr != nil {
			// 【already active 不是失败】子进程是长驻桥:会话首轮之后就一直"活跃"
			// 在桥内,而 0.2.0-rc.2 对活跃会话的 resume 一律拒绝(proto020 实测
			// 2026-10-02:-32602 "Invalid params: session is already active: <id>",
			// 活跃判定先于 cwd 比对)。此前直接把错误上抛,设置页"该会话实时原生
			// 设置"在首轮后必然失败。与 remountSession 同一判别式:命中 already
			// active 按会话存活处理,目录回**最近一次已知快照**(每次 session/new|
			// resume|config_option_update 都刷新,是该会话目录的最近地面真值);
			// 从未取样过(无快照)才显式报错,不伪造目录。
			var re *dshapp.RPCError
			if errors.As(rerr, &re) && re.Code == -32602 && strings.Contains(re.Message, "session is already active") {
				snap := p.rawCatalogSnapshot()
				if len(snap) == 0 {
					return fmt.Errorf("session/resume %s: %w(会话在本桥内活跃,尚无已知的原生目录快照,无法呈现)", sid, rerr)
				}
				log.Printf("[dsh] 会话 %s 已在本桥内活跃(resume 被拒),原生设置回最近已知目录快照(%d 项)", sid, len(snap))
				opts = snap
				return nil
			}
			return fmt.Errorf("session/resume %s: %w", sid, rerr)
		}
		opts = st.ConfigOptions
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(opts) > 0 {
		p.refreshCatalogFromResume(opts)
	}
	return dshOptionsToRows(opts), nil
}

// dshSessionIDFor 取线程的 dsh 会话 id:同代内存缓存优先,回落 ExternalSession。
func (p *DshProvider) dshSessionIDFor(t *store.Thread) string {
	gen := int(p.gen.Load())
	if ref, hit := p.dshThreadRefFor(t.ID); hit && ref.gen == gen {
		return ref.id
	}
	if t.AgentID == "" || t.AgentID == "dsh" {
		return t.ExternalSession.SessionID
	}
	return ""
}

// dshCatalogProbe 是一次目录取样探测的在飞状态(结果先写后关 done;dshModelsProbe
// 同款:等待方看到 done 关闭时结果必然可见)。
type dshCatalogProbe struct {
	done chan struct{}
	opts []dshapp.ConfigOption
	err  error
}

// sampleCatalogOptions 用一次性瞬态会话取目录(新建 → 灌"新会话初始值"(已确认
// 灌入的项覆盖 currentValue)→ close,全程不作业(临时目录)),并发调用单飞去重
// (与 fetchModels 同款治理,2026-10-01 探测互踩治理:设置页并发打开/写入校验时
// 共享同一次瞬态会话取样,不在 lifeMu 上互踩)。等待方共享领队结果;自身 ctx 先
// 到截止则放弃(领队继续探测,不受等待方影响)。
func (p *DshProvider) sampleCatalogOptions(ctx context.Context) ([]dshapp.ConfigOption, error) {
	return p.sampleCatalogSingleFlight(ctx, p.sampleCatalogUncached)
}

// sampleCatalogSingleFlight 把 do 包装成并发去重:第一个调用者执行 do,在飞期间
// 的其余调用者等待并共享其结果(含失败)。测试注入 do 以钉住"N 个并发调用只跑 1 次"。
func (p *DshProvider) sampleCatalogSingleFlight(ctx context.Context, do func(context.Context) ([]dshapp.ConfigOption, error)) ([]dshapp.ConfigOption, error) {
	p.catProbeMu.Lock()
	if cur := p.catProbe; cur != nil {
		p.catProbeMu.Unlock()
		select {
		case <-cur.done:
			return cur.opts, cur.err
		case <-ctx.Done():
			return nil, fmt.Errorf("等待在飞目录探测: %w", ctx.Err())
		}
	}
	f := &dshCatalogProbe{done: make(chan struct{})}
	p.catProbe = f
	p.catProbeMu.Unlock()

	f.opts, f.err = do(ctx)
	close(f.done)

	// 只清自己名下的在飞槽:若已有新调用换上了新的 f,不要误删别人的。
	p.catProbeMu.Lock()
	if p.catProbe == f {
		p.catProbe = nil
	}
	p.catProbeMu.Unlock()
	return f.opts, f.err
}

// sampleCatalogUncached 是目录取样的真实实现(无去重,经 sampleCatalogSingleFlight
// 调用)。先礼让在飞握手(有界)再开自己的取样预算,理由同 fetchModelsUncached
// (预算不能被等锁期烧光)。
func (p *DshProvider) sampleCatalogUncached(ctx context.Context) ([]dshapp.ConfigOption, error) {
	p.awaitHandshake(dshBusyHandshakeWait)
	ctx, cancel := context.WithTimeout(ctx, dshCatalogBudget)
	defer cancel()
	var opts []dshapp.ConfigOption
	err := p.withClientCtx(ctx, dshInitBudget, func(ctx context.Context, c dshClient) error {
		st, err := c.SessionNew(ctx, os.TempDir())
		if err != nil {
			return err
		}
		opts = st.ConfigOptions
		// 灌"新会话初始值"(尽力而为,单项失败不中断 —— currentValue 如实反映
		// 灌入成功的项;失败只落日志,不伪造成功)。
		defaults := p.nativeDefaultsSnapshot()
		applied := map[string]string{}
		for _, id := range dshSortedDefaultIDs(defaults) {
			if err := c.SetConfigOption(ctx, st.SessionID, id, defaults[id]); err != nil {
				log.Printf("[dsh] 灌新会话初始值 %s=%s 失败(跳过): %v", id, defaults[id], err)
				continue
			}
			applied[id] = defaults[id]
		}
		opts = dshOverlayCurrent(opts, applied)
		closeCtx, cc := context.WithTimeout(context.Background(), dshInitBudget)
		defer cc()
		_ = c.SessionClose(closeCtx, st.SessionID) // 用完即毁
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(opts) > 0 {
		p.catMu.Lock()
		p.setRawCatalog(opts)
		p.catMu.Unlock()
	}
	return opts, nil
}

// dshSortedDefaultIDs 稳定顺序灌入(map 迭代无序,逐项写入顺序必须确定)。
func dshSortedDefaultIDs(defaults map[string]string) []string {
	ids := make([]string, 0, len(defaults))
	for id := range defaults {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// dshOverlayCurrent 用已确认灌入的值覆盖 currentValue(其余项原样保留)。
func dshOverlayCurrent(opts []dshapp.ConfigOption, applied map[string]string) []dshapp.ConfigOption {
	if len(applied) == 0 || len(opts) == 0 {
		return opts
	}
	out := append([]dshapp.ConfigOption(nil), opts...)
	for i := range out {
		if v, ok := applied[out[i].ID]; ok {
			out[i].CurrentValue = v
		}
	}
	return out
}

// dshOptionsToRows 原生目录 → 展示行(纯透传:候选经 FlattenOptionLeaves 展平,
// value 逐字节原样;原生形状不认识时该行无候选,不伪造)。
func dshOptionsToRows(opts []dshapp.ConfigOption) []NativeConfigOption {
	rows := make([]NativeConfigOption, 0, len(opts))
	for _, o := range opts {
		row := NativeConfigOption{
			ID:           o.ID,
			Name:         o.Name,
			Category:     o.Category,
			Type:         o.Type,
			CurrentValue: o.CurrentValue,
			Writable:     true,
			Note:         dshOptionNote,
		}
		for _, leaf := range dshapp.FlattenOptionLeaves(o.Options) {
			row.Options = append(row.Options, NativeConfigOptionValue{
				Value: leaf.Value, Name: leaf.Name, Description: leaf.Description, Group: leaf.Group,
			})
		}
		rows = append(rows, row)
	}
	return rows
}

// SetNativeConfigOption 实现 agent.NativeConfigOptions。
//
// 【校验先行】optionID 必须在目录里、value 必须与该目录某候选叶子**逐字节相等**
// (红线:禁止裸模型名 —— dsh model 的取值是双层 JSON 编码串,任何现拼都是污染;
// 目录缺失时先刷新一次再找,桌面端目录可能刚变)。
func (p *DshProvider) SetNativeConfigOption(ctx context.Context, t *store.Thread, optionID, value string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	opts := p.rawCatalogSnapshot()
	if !dshOptionHasValue(opts, optionID, value) {
		// 目录缺失/过期:刷新一次再校验(与 applyConfigOptions 的目录兜底同语义)。
		fresh, err := p.sampleCatalogOptions(ctx)
		if err != nil {
			return fmt.Errorf("原生设置目录不可用,无法校验取值: %w", err)
		}
		opts = fresh
	}
	if !dshOptionHasValue(opts, optionID, value) {
		return fmt.Errorf("取值不在原生目录候选内(禁止透传目录外的值): %s", optionID)
	}

	if t == nil {
		// 无线程:记"新会话初始值"(探针实证 set_config_option 仅会话内生效,
		// 无会话可当场写;新会话创建时由 ensureSession 灌入)。
		p.catMu.Lock()
		if p.nativeDefaults == nil {
			p.nativeDefaults = map[string]string{}
		}
		p.nativeDefaults[optionID] = value
		p.catMu.Unlock()
		log.Printf("[dsh] 原生设置初始值已记录: %s=%s(新会话生效)", optionID, value)
		return nil
	}
	if p.turnActive.Load() {
		return ErrBusy
	}
	sid := p.dshSessionIDFor(t)
	if sid == "" {
		return errors.New("该会话尚未建立 dsh 原生会话,无法当场生效(可先发起一轮对话)")
	}
	return p.withClientCtx(ctx, dshInitBudget, func(ctx context.Context, c dshClient) error {
		if err := c.SetConfigOption(ctx, sid, optionID, value); err != nil {
			return fmt.Errorf("set_config_option(%s): %w", optionID, err)
		}
		return nil
	})
}

// dshOptionHasValue 校验 optionID 在目录里且 value 与某候选叶子逐字节相等。
// 候选为空的行(原生没报候选)一律拒绝写 —— 没有可核对的白名单就不放行。
func dshOptionHasValue(opts []dshapp.ConfigOption, optionID, value string) bool {
	for _, o := range opts {
		if o.ID != optionID {
			continue
		}
		for _, leaf := range dshapp.FlattenOptionLeaves(o.Options) {
			if leaf.Value == value {
				return true
			}
		}
	}
	return false
}
