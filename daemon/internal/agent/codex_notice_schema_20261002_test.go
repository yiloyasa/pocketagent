// codex_notice_schema_20261002_test.go —— 2026-10-02 深接入差距修复批的钉子。
//
// 【钉什么】codexGlobalNoticeKinds(登记表)与 vendored 0.159.2 schema 的双向对表:
//  1. 【幻影反例】登记表里每个方法名都必须在 ServerNotification.json 的 oneOf
//     里真实存在 —— 幻影方法(thread/plan/updated 前科)当场拦下,不再靠活体探针
//     事后发现;
//  2. 【覆盖完整】schema 里每个通知方法,要么已登记、要么逐个落在"明确不收清单"
//     (桌面产品面,如实留 UNKNOWN)—— 上游新增方法时本测试即红,强迫做一次
//     有意识的决定,而不是默默灌 UNKNOWN;
//  3. 【分类正确】本批补登的 16 个已投影方法必须 PROJECTED,"明确不收"的必须
//     UNKNOWN;
//  4. 【词表不脱节】每个已登记方法经 TranslateEvent 必须出人话、不得译成未知
//     (分类/翻译两张皮在同一张表上对齐);
//  5. 【单表】轮内兜底分类(codexMethodNoticeKind)与全局表同一定性,不再双表漂移。
//
// 【证据】活体轮探针 docs/verify/evidence/codex-01592-liveturn-20261002.log;
// schema 快照为 codex-schema/ServerNotification.json(0.159.2)。
package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	xruntime "runtime"

	"github.com/pocketagent/daemon/internal/runtime"
)

// codexNoticeUnclaimedAllowlist 是"明确不收清单"的**穷举集**(knowledge_codex.go
// 文件头同源):schema 里有、桌面产品面、手机端没有消费方,出现即如实 UNKNOWN。
// 它必须与"schema 全集 − 登记表"严格相等(双向钉,见主测试)。
var codexNoticeUnclaimedAllowlist = map[string]bool{
	"thread/attachment/updated":             true, // daemon 无附件特性
	"project/changed":                       true, // 桌面 Projects 面
	"thread/project/updated":                true, // 桌面 Projects 面(会话↔项目绑定)
	"mcpServer/event/stream/notification":   true, // 桌面 MCP 流(留观)
	"account/gatewayOAuth/changed":          true, // 桌面 OAuth 网关
	"app/list/updated":                      true, // 桌面应用管理
	"externalAgentConfig/import/progress":   true, // 桌面导入
	"externalAgentConfig/import/completed":  true, // 桌面导入
	"fs/changed":                            true, // 桌面文件监视
	"fuzzyFileSearch/sessionUpdated":        true, // 桌面模糊搜索
	"fuzzyFileSearch/sessionCompleted":      true, // 桌面模糊搜索
	"thread/realtime/started":               true, // 语音实时面(11 条族)
	"thread/realtime/itemAdded":             true,
	"thread/realtime/item/started":          true,
	"thread/realtime/item/transcript/delta": true,
	"thread/realtime/item/completed":        true,
	"thread/realtime/transcript/delta":      true,
	"thread/realtime/transcript/done":       true,
	"thread/realtime/outputAudio/delta":     true,
	"thread/realtime/sdp":                   true,
	"thread/realtime/error":                 true,
	"thread/realtime/closed":                true,
	"windowsSandbox/setupCompleted":         true, // Windows 沙箱安装向导
}

// loadServerNotificationMethods 从 vendored schema 抽出 ServerNotification oneOf
// 里的全部 method 名。路径以本测试文件为锚(codex_goal_test.go 的 loadGoalSchema
// 同款),不依赖 go test 的工作目录。
func loadServerNotificationMethods(t *testing.T) []string {
	t.Helper()
	_, thisFile, _, ok := xruntime.Caller(0)
	if !ok {
		t.Fatal("拿不到测试文件路径")
	}
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "codex-schema", "ServerNotification.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读 schema 失败(%s): %v —— 对表测试依赖仓库内 codex-schema 快照", path, err)
	}
	var s struct {
		OneOf []struct {
			Properties struct {
				Method struct {
					Const string   `json:"const"`
					Enum  []string `json:"enum"`
				} `json:"method"`
			} `json:"properties"`
		} `json:"oneOf"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("schema 解析失败(%s): %v", path, err)
	}
	var out []string
	for _, branch := range s.OneOf {
		name := branch.Properties.Method.Const
		if name == "" && len(branch.Properties.Method.Enum) > 0 {
			name = branch.Properties.Method.Enum[0]
		}
		if name != "" {
			out = append(out, name)
		}
	}
	if len(out) < 80 {
		t.Fatalf("schema 解析出的通知方法过少(%d)—— 快照形状可能变了,先核对 codex-schema", len(out))
	}
	return out
}

// TestCodexNoticeRegistryMatchesVendoredSchema 登记表与 schema 的双向对表(幻影反例)。
func TestCodexNoticeRegistryMatchesVendoredSchema(t *testing.T) {
	schemaSet := map[string]bool{}
	for _, m := range loadServerNotificationMethods(t) {
		schemaSet[m] = true
	}
	// ① 幻影反例:登记表里的每个名字都必须真实存在于 0.159.2 schema。
	for method := range codexGlobalNoticeKinds {
		if !schemaSet[method] {
			t.Errorf("登记表方法 %q 不在 vendored 0.159.2 ServerNotification 里(幻影?codexplanprobe 的 thread/plan/updated 前科)", method)
		}
	}
	// ② 覆盖完整:schema 里每个名字要么已登记,要么逐个落在明确不收清单。
	for m := range schemaSet {
		if _, registered := codexGlobalNoticeKinds[m]; registered {
			continue
		}
		if !codexNoticeUnclaimedAllowlist[m] {
			t.Errorf("schema 方法 %q 既未登记也不在明确不收清单 —— 新通知必须做一次有意识的决定(收编或入清单),不能默默灌 UNKNOWN", m)
		}
	}
	// ③ 清单不养闲人:allowlist 里的名字必须真实在 schema 里(防清单腐化)。
	for m := range codexNoticeUnclaimedAllowlist {
		if !schemaSet[m] {
			t.Errorf("明确不收清单里的 %q 已不在 0.159.2 schema 中 —— 从清单移除或核实快照版本", m)
		}
	}
	// ④ 分类值合法:登记表只能出现三档 runtime 词表值。
	valid := map[string]bool{
		runtime.KindKnownProjected:      true,
		runtime.KindKnownPassthrough:    true,
		runtime.KindUnknownNotification: true,
	}
	for method, kind := range codexGlobalNoticeKinds {
		if !valid[kind] {
			t.Errorf("登记表 %q 的分类 %q 不是合法档位", method, kind)
		}
	}
}

// TestCodexGlobalNoticeKind20261002Batch 本批补登的 16 个已投影方法必须 PROJECTED,
// 明确不收的必须 UNKNOWN(与登记表逐条对齐,防止表被人改了测试还绿着)。
func TestCodexGlobalNoticeKind20261002Batch(t *testing.T) {
	projected := []string{
		"error", "guardianWarning",
		"item/autoApprovalReview/started", "item/autoApprovalReview/completed",
		"autoApprovalReview/strictReviewRequired",
		"item/plan/delta",
		"command/exec/outputDelta", "process/outputDelta",
		"item/fileChange/outputDelta", "item/fileChange/patchUpdated",
		"item/mcpToolCall/progress",
		"model/rerouted", "model/safetyBuffering/updated",
		"thread/compacted", "item/reasoning/textDelta",
		"windows/worldWritableWarning",
	}
	for _, m := range projected {
		if got := codexGlobalNoticeKind(m); got != runtime.KindKnownProjected {
			t.Errorf("codexGlobalNoticeKind(%q) = %q, want PROJECTED(事件翻译层有投影分支)", m, got)
		}
	}
	for m := range codexNoticeUnclaimedAllowlist {
		if got := codexGlobalNoticeKind(m); got != runtime.KindUnknownNotification {
			t.Errorf("明确不收清单 %q 被登记为 %q —— 桌面面必须如实留 UNKNOWN", m, got)
		}
	}
}

// TestCodexRegisteredNoticesTranslateToPlainWords 词表不脱节:每个已登记方法经
// TranslateEvent 必须出人话、不得译成"未知"(本批 +14 词条的全量兜底钉)。
func TestCodexRegisteredNoticesTranslateToPlainWords(t *testing.T) {
	for method, kind := range codexGlobalNoticeKinds {
		tr := runtime.TranslateEvent("codex", method, kind, `{"threadId":"th-schema"}`)
		if tr.Summary == "" {
			t.Errorf("%s: 词表缺条目,summary 为空", method)
			continue
		}
		if strings.Contains(strings.ToLower(tr.Summary), "未知") {
			t.Errorf("%s: 已登记方法不得译成未知事件: %s", method, tr.Summary)
		}
	}
}

// TestCodexMethodNoticeKindUsesSameTable 轮内兜底与全局表同一定性(单表,不漂移)。
func TestCodexMethodNoticeKindUsesSameTable(t *testing.T) {
	for method := range codexGlobalNoticeKinds {
		if got, want := codexMethodNoticeKind(method), codexGlobalNoticeKind(method); got != want {
			t.Errorf("codexMethodNoticeKind(%q) = %q, 全局表 = %q —— 两张表漂移了", method, got, want)
		}
	}
	// 表外方法两路同样落 UNKNOWN(不猜;thread/plan/updated 是定案的幻影)。
	for _, m := range []string{"futureDept/codexProbe", "thread/plan/updated"} {
		if got := codexMethodNoticeKind(m); got != runtime.KindUnknownNotification {
			t.Errorf("表外方法 %q 应落 UNKNOWN,得到 %q", m, got)
		}
	}
}
