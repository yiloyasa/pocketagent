package agent

// capability_gate_test.go —— 能力位全集守护测试(与 docs/2026-09-30-agent-capability-matrix.md 对表)。
//
// 【钉住什么】daemon 侧能力位一律在 internal/server/host.go 的 GetHostInfo
// capabilities 段按接口断言生成(host.go:75-155),位名是**字面量**、无常量可用;
// agent 包被 server 依赖,反向 import 会成环 —— 于是本测试用两条腿把「能力位全集
// 与文档矩阵一致」钉死:
//
//	腿一(词汇表,TestCapabilityBitVocabularyMatchesMatrix):解析 host.go 里
//	  `caps = append(caps, ...)` 的字面量,断言「agent 域位族 + 宿主级位」与测试内
//	  固化的期望表一致,并要求矩阵文档逐位族收录。host.go 增删位族时本测试红,
//	  失败信息直接提醒同步矩阵文档。
//	腿二(逐家表态,TestCapabilityStanceTableMatchesMatrix):用真实四家构造器 +
//	  与 host.go 同形状的窄接口断言(Go 接口按形状满足,host.go 文件头同款纪律),
//	  验证每家每位的"有/无"与矩阵 A 区逐格一致。provider 悄悄丢实现/翻表态时
//	  本测试红(server 侧 host_capability_bits_test.go 从 GetHostInfo 端到端钉
//	  同一件事,两侧任一断都有人报警)。
//
// 【为什么不直接调 GetHostInfo】那要 import internal/server,成环;腿二的结构性
// 接口断言与 host.go 的生成判据一一对应,等价且零耦合。

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/store"
)

// capabilityBitFamilies 期望的 agent 域位族全集(格式 `<族>:<agentId>`)。
// "agent" 是成员位(注册即有,pi 追加版本号),由 agentCapability(id) 生成。
// 期望表来源:docs/2026-09-30-agent-capability-matrix.md 第二节主矩阵 A(2026-09-30,
// 基线 a5f3f3e)。host.go 位族增删时:先更新矩阵文档,再回这里改期望表 —— 两处
// 不同步,本测试就用失败信息把人拽回来。
var capabilityBitFamilies = []string{
	"agent",
	"agent_settings",
	"agent_native_settings",
	"text_generation",
	"collab",
	"steer",
	"queue_clear",
	"compact",
	"fork",
	"rollback",
	"review",
	"export_html",
	"image_input",
}

// capabilityHostBits 期望的宿主级位(无 agentId 后缀,循环外固定追加)。
var capabilityHostBits = []string{
	"terminal", "fs", "git", "agent", "event_sequence", "runtime.v1",
}

const capabilityMatrixDoc = "docs/2026-09-30-agent-capability-matrix.md"

// hostGoSource 定位并读出 internal/server/host.go 源码(测试文件在 agent 包,
// host.go 在 ../../internal/server/)。
func hostGoSource(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位本测试文件路径(runtime.Caller 失败)")
	}
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "internal", "server", "host.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读不到能力位生成源码 %s: %v", path, err)
	}
	return string(src)
}

// extractCapabilityAppends 从 host.go 源码提取 capabilities 生成段的字面量,
// 按「成员位 / agent 域位族 / 宿主级位」三分类返回。
//
// 识别规则(对 host.go 现有写法的最小约定,写法漂移会体现在返回集合的变化里,
// 由期望表对表兜住):
//   - 行含 `caps = append(caps, agentCapability(id)` → 成员位;
//   - 行含 `caps = append(caps, "xxx:"+id)` → 位族 xxx(字面量以冒号结尾);
//   - 其余 `caps = append(caps, ...)` 行里的字面量 → 宿主级位。
func extractCapabilityAppends(src string) (hasMembership bool, families, hostBits map[string]bool) {
	families = map[string]bool{}
	hostBits = map[string]bool{}
	literal := regexp.MustCompile(`"([^"]+)"`)
	const marker = "caps = append(caps,"
	for _, line := range strings.Split(src, "\n") {
		idx := strings.Index(line, marker)
		if idx < 0 {
			continue
		}
		payload := strings.TrimSuffix(strings.TrimSpace(line[idx+len(marker):]), ")")
		switch {
		case strings.Contains(payload, "agentCapability("):
			hasMembership = true
			families["agent"] = true // 成员位 `agent:<id>`(pi 追加版本号)也算一个位族
		case strings.Contains(payload, "+ id"), strings.Contains(payload, "+id"):
			for _, m := range literal.FindAllStringSubmatch(payload, -1) {
				if strings.HasSuffix(m[1], ":") {
					families[strings.TrimSuffix(m[1], ":")] = true
				}
			}
		default:
			for _, m := range literal.FindAllStringSubmatch(payload, -1) {
				hostBits[m[1]] = true
			}
		}
	}
	return hasMembership, families, hostBits
}

func TestCapabilityBitVocabularyMatchesMatrix(t *testing.T) {
	src := hostGoSource(t)
	hasMembership, families, hostBits := extractCapabilityAppends(src)

	// 成员位:agentCapability(id) 必须还在生成段里(pi 版本号格式随它走)。
	if !hasMembership {
		t.Fatal("host.go 不再经 agentCapability(id) 生成成员位 `agent:<id>` —— 成员位生成路径被改,请核对 docs/2026-09-30-agent-capability-matrix.md 第二节并同步本测试期望表")
	}

	// 位族与宿主级位:双向往集合相等(代码多的位=没进文档的位;文档多的位=代码里已删的位)。
	assertSetEquals(t, "agent 域位族(host.go ↔ 期望表)", families, setOf(capabilityBitFamilies))
	assertSetEquals(t, "宿主级位(host.go ↔ 期望表)", hostBits, setOf(capabilityHostBits))

	// 文档对表:每个位族/宿主级位必须被矩阵文档收录(文档路径以 `<族>:` 形态出现;
	// 宿主级位按裸 token 查)。文档缺失/改名同样红 —— 守护测试存在的意义就是
	// "代码与文档永远一起动"。
	docPath := filepath.Join("..", "..", "..", filepath.FromSlash(capabilityMatrixDoc))
	doc, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("读不到能力位矩阵文档 %s: %v(文档被改名/移动时请同步本测试)", capabilityMatrixDoc, err)
	}
	docText := string(doc)
	for _, family := range capabilityBitFamilies {
		if !strings.Contains(docText, family+":") {
			t.Errorf("矩阵文档 %s 未收录位族 `%s:` —— host.go 已生成该族,文档必须逐族给出四家口径", capabilityMatrixDoc, family)
		}
	}
	for _, bit := range capabilityHostBits {
		if !strings.Contains(docText, "`"+bit+"`") {
			t.Errorf("矩阵文档 %s 未收录宿主级位 `%s`", capabilityMatrixDoc, bit)
		}
	}
}

// TestCapabilityStanceTableMatchesMatrix 四家 provider 逐位表态对表(矩阵 A 区逐格)。
//
// 【判定与 host.go 同判据】host.go 生成位走接口断言(部分位还要方法真值),本测试
// 用同形状的窄接口/方法调用复刻同一判定;期望值 = 矩阵 A 区该格的口径。collab 的
// CollabReady() 是运行时接线真值(pi 看本机目录、dsh 看会话级注入),hermetic 起见
// 这里只钉接口归属,真值语义由 collab_ready_test.go 钉。
func TestCapabilityStanceTableMatchesMatrix(t *testing.T) {
	providers := map[string]Provider{
		"codex": NewCodexProvider(),
		"pi":    NewPiProvider(),
		"zcode": NewZcodeProvider(),
		"dsh":   NewDshProvider(),
	}

	// compact 的三种原生形状(host.go:121-129 同款,参数类型互斥、一家只命中一种)。
	compactShaped := func(p Provider) bool {
		if _, ok := p.(interface{ CompactThread(string) error }); ok {
			return true
		}
		if _, ok := p.(interface{ CompactThread(*store.Thread) error }); ok {
			return true
		}
		_, ok := p.(interface{ CompactSession(string) error })
		return ok
	}
	// image_input:断言通过还要调方法(host.go:150 同款 —— 表态值是实现自己声明的)。
	imageInputShaped := func(p Provider) bool {
		ii, ok := p.(ImageInputter)
		return ok && ii.SupportsImageInput()
	}

	stances := []struct {
		family string // 位族名(矩阵 A 区行)
		probe  func(Provider) bool
		want   map[string]bool // 四家期望表态(=矩阵 A 区该行四格)
	}{
		{
			// 成员位:注册即有;四家构造器在手即视同注册(server 侧端到端由
			// host_capability_bits_test.go 覆盖)。
			family: "agent",
			probe:  func(Provider) bool { return true },
			want:   map[string]bool{"codex": true, "pi": true, "zcode": true, "dsh": true},
		},
		{
			family: "agent_settings",
			probe:  func(p Provider) bool { _, ok := p.(PrefsAware); return ok },
			want:   map[string]bool{"codex": true, "pi": true, "zcode": true, "dsh": true},
		},
		{
			family: "agent_native_settings",
			probe:  func(p Provider) bool { _, ok := p.(NativeConfigOptions); return ok },
			want:   map[string]bool{"codex": false, "pi": true, "zcode": false, "dsh": true},
		},
		{
			family: "text_generation",
			probe:  func(p Provider) bool { _, ok := p.(TextGenerator); return ok },
			want:   map[string]bool{"codex": true, "pi": true, "zcode": true, "dsh": false},
		},
		{
			family: "collab",
			// 只钉接口归属(host.go:93 的位生成 = CollabAware 断言 **且** CollabReady()
			// 真值;真值依赖运行时接线 —— pi 看 daemon 自有目录、dsh 看部署级补丁层、
			// codex/zcode/hermes 看 collab_inject.go 的注入要素 —— hermetic 环境会漂,
			// 由 collab_ready_test.go / collab_inject_test.go 单独钉)。X2 收口后五家
			// 接口全部在场:codex(-c argv 覆盖)/zcode(会话参数)/hermes(ACP 会话参数)
			// 的注入通道实证见 collab_inject.go 文件头。
			probe: func(p Provider) bool { _, ok := p.(CollabAware); return ok },
			want:  map[string]bool{"codex": true, "pi": true, "zcode": true, "dsh": true},
		},
		{
			family: "steer",
			probe:  func(p Provider) bool { _, ok := p.(Steerer); return ok },
			want:   map[string]bool{"codex": true, "pi": true, "zcode": false, "dsh": false},
		},
		{
			family: "queue_clear",
			probe:  func(p Provider) bool { _, ok := p.(QueueClearer); return ok },
			want:   map[string]bool{"codex": false, "pi": true, "zcode": false, "dsh": false},
		},
		{
			family: "compact",
			probe:  compactShaped,
			want:   map[string]bool{"codex": true, "pi": true, "zcode": true, "dsh": false},
		},
		{
			family: "fork",
			// fork 的两种原生形状(host.go:152-157 同款:codex (string,string) 形
			// 与 pi *store.Thread 形互斥、任一在场即声明同一位;与 compact 三形状
			// 一位同一先例)。【2026-10-14 更正】此前只探 codex 形、期望 pi=false,
			// 与 host.go 已生成 fork:pi(server/host_capability_bits_test.go:83-87
			// 真实构造器钉死)及矩阵 pi 格✅ 相悖 —— 漏探形状属漂移,随矩阵 pi fork
			// 格更正一并改(证据 pi.go:1878 ForkThread + host.go:155 piShapedForker
			// + pi_fork_test.go)。
			probe: func(p Provider) bool {
				if _, ok := p.(interface {
					ForkThread(string, string) (string, error)
				}); ok {
					return true
				}
				_, ok := p.(interface {
					ForkThread(*store.Thread) (store.ExternalSessionRef, error)
				})
				return ok
			},
			want: map[string]bool{"codex": true, "pi": true, "zcode": false, "dsh": false},
		},
		{
			family: "rollback",
			probe: func(p Provider) bool {
				_, ok := p.(interface {
					RollbackThread(string, int32) error
				})
				return ok
			},
			want: map[string]bool{"codex": true, "pi": false, "zcode": false, "dsh": false},
		},
		{
			family: "review",
			probe: func(p Provider) bool {
				_, ok := p.(interface {
					StartReview(*store.Thread, string) (*CodexReviewStart, error)
				})
				return ok
			},
			want: map[string]bool{"codex": true, "pi": false, "zcode": false, "dsh": false},
		},
		{
			family: "export_html",
			probe: func(p Provider) bool {
				_, ok := p.(interface {
					ExportHTML(*store.Thread) (string, error)
				})
				return ok
			},
			want: map[string]bool{"codex": false, "pi": true, "zcode": false, "dsh": false},
		},
		{
			family: "image_input",
			probe:  imageInputShaped,
			want:   map[string]bool{"codex": true, "pi": true, "zcode": true, "dsh": false},
		},
	}

	for _, tc := range stances {
		for _, id := range []string{"codex", "pi", "zcode", "dsh"} {
			got := tc.probe(providers[id])
			want, expected := tc.want[id]
			if !expected {
				t.Fatalf("位族 %s 的期望表漏了 agent %s(表驱动必须四家全列)", tc.family, id)
			}
			if got != want {
				t.Errorf("%s 的 %s 位表态漂移:got %v want %v(host.go 生成判据或 provider 实现变了 —— 先核对 docs/2026-09-30-agent-capability-matrix.md 第二节该格,再决定改实现还是改矩阵/期望表)", id, tc.family, got, want)
			}
		}
	}
}

// ---- 小工具 ----

func setOf(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}

func assertSetEquals(t *testing.T, what string, got, want map[string]bool) {
	t.Helper()
	var extra, missing []string
	for k := range got {
		if !want[k] {
			extra = append(extra, k)
		}
	}
	for k := range want {
		if !got[k] {
			missing = append(missing, k)
		}
	}
	if len(extra) > 0 || len(missing) > 0 {
		t.Fatalf("%s 不一致,请先同步 docs/2026-09-30-agent-capability-matrix.md 再改本测试期望表:多出 %v / 缺少 %v", what, extra, missing)
	}
}
