package server

// collab_roles_test.go —— 协同角色(分工编排)的回归用例。
//
// 【为什么钉这些】角色表是**引用性配置**:id 一旦和实际落库值对不上(多了空格、
// 被静默截断、跨重启丢失),症状就是"我明明配了却引用不到",而且极难归因 ——
// 因为报错发生在委派那一刻,离配置界面很远。所以这里把**每一条可能让 id 悄悄
// 变形的路径**都钉死:trim、校验、拒绝含空格、超限报错、重启后仍在。

import (
	"context"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

func roleFixture(id string) *pb.CollabRole {
	return &pb.CollabRole{
		Id:           id,
		Name:         "安全审查者",
		AgentId:      "pi",
		Description:  "只找安全问题,不修改任何代码",
		Instructions: "必须列出证据文件路径与行号",
		Model:        "pi/cheap",
		Effort:       "low",
	}
}

// ① CRUD 往返 + 列表顺序稳定 + 删除幂等。
func TestCollabRoleCRUDRoundTrip(t *testing.T) {
	s := newTestServer(t)

	if _, err := s.SetCollabRole(context.Background(), &pb.SetCollabRoleRequest{Role: roleFixture("zeta")}); err != nil {
		t.Fatalf("新建角色失败: %v", err)
	}
	if _, err := s.SetCollabRole(context.Background(), &pb.SetCollabRoleRequest{Role: roleFixture("alpha")}); err != nil {
		t.Fatalf("新建角色失败: %v", err)
	}

	got, err := s.ListCollabRoles(context.Background(), &pb.ListCollabRolesRequest{})
	if err != nil {
		t.Fatalf("列出角色失败: %v", err)
	}
	if len(got.GetRoles()) != 2 {
		t.Fatalf("应有 2 个角色,实际 %d", len(got.GetRoles()))
	}
	// 顺序必须稳定(底层是 map,不排序每次都会抖,手机列表会跟着抖)。
	if got.GetRoles()[0].GetId() != "alpha" || got.GetRoles()[1].GetId() != "zeta" {
		t.Fatalf("角色应按 id 升序,实际: %s, %s", got.GetRoles()[0].GetId(), got.GetRoles()[1].GetId())
	}
	// 字段往返不能丢:模型/思考强度/长期要求正是这个功能的价值所在。
	z := got.GetRoles()[1]
	if z.GetModel() != "pi/cheap" || z.GetEffort() != "low" || z.GetInstructions() != "必须列出证据文件路径与行号" {
		t.Fatalf("角色字段往返丢失: model=%q effort=%q instructions=%q", z.GetModel(), z.GetEffort(), z.GetInstructions())
	}

	if _, err := s.DeleteCollabRole(context.Background(), &pb.DeleteCollabRoleRequest{Id: "zeta"}); err != nil {
		t.Fatalf("删除角色失败: %v", err)
	}
	// 幂等:删一个不存在的 id 不该报错(目标状态已达成)。
	if _, err := s.DeleteCollabRole(context.Background(), &pb.DeleteCollabRoleRequest{Id: "zeta"}); err != nil {
		t.Fatalf("重复删除应幂等成功,却报错: %v", err)
	}
	rest, _ := s.ListCollabRoles(context.Background(), &pb.ListCollabRolesRequest{})
	if len(rest.GetRoles()) != 1 || rest.GetRoles()[0].GetId() != "alpha" {
		t.Fatalf("删除后应只剩 alpha,实际 %v", idsOf(rest))
	}
}

// ② 真持久化:关掉 store 再打开,角色还在(配置丢了就等于没配)。
// newRoleTestPair 自建 server 并**保留数据目录**:重开持久化用例要用它。
// newTestServer 不暴露目录,而这里恰恰需要一个能重开的路径。
func newRoleTestPair(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	fs, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	return &Server{cfg: Config{Store: fs}}, dir
}

func TestCollabRolesSurviveReopen(t *testing.T) {
	s, dir := newRoleTestPair(t)
	if _, err := s.SetCollabRole(context.Background(), &pb.SetCollabRoleRequest{Role: roleFixture("reviewer")}); err != nil {
		t.Fatal(err)
	}
	// 复用同一个数据目录重建 FileStore,模拟 daemon 重启。
	fs2, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatalf("重开存储失败: %v", err)
	}
	s2 := &Server{cfg: Config{Store: fs2}}
	got, err := s2.ListCollabRoles(context.Background(), &pb.ListCollabRolesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.GetRoles()) != 1 || got.GetRoles()[0].GetId() != "reviewer" {
		t.Fatalf("重启后角色丢失: %v", idsOf(got))
	}
}

// ③ id 的三条禁令:空 / 含空格 / 超长,都必须**人话报错**而不是悄悄改。
func TestCollabRoleRejectsBadID(t *testing.T) {
	s := newTestServer(t)
	cases := []struct {
		name string
		id   string
		want string
	}{
		{"空 id", "", "id 不能为空"},
		{"含空格", "my role", "不能含空格"},
		{"超长", strings.Repeat("x", 200), "id 过长"},
	}
	for _, c := range cases {
		r := roleFixture(c.id)
		_, err := s.SetCollabRole(context.Background(), &pb.SetCollabRoleRequest{Role: r})
		if err == nil {
			t.Fatalf("%s: 应报错却通过了", c.name)
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: 错误信息应含 %q,实际: %v", c.name, c.want, err)
		}
	}
	// 出错的都没落库。
	got, _ := s.ListCollabRoles(context.Background(), &pb.ListCollabRolesRequest{})
	if len(got.GetRoles()) != 0 {
		t.Fatalf("非法角色不该落库: %v", idsOf(got))
	}
}

// ④ trim 只发生在 id/值类字段上,instructions 必须原样保留(它有排版语义)。
func TestCollabRoleTrimSemantics(t *testing.T) {
	s := newTestServer(t)
	r := roleFixture("  spaced  ")
	r.Instructions = "  缩进是有意义的"
	resp, err := s.SetCollabRole(context.Background(), &pb.SetCollabRoleRequest{Role: r})
	if err != nil {
		t.Fatalf("入参含空格应被 trim 后接受: %v", err)
	}
	if resp.GetRole().GetId() != "spaced" {
		t.Fatalf("回显应是 trim 后的真值,实际 %q", resp.GetRole().GetId())
	}
	if r.Instructions == "" {
		t.Fatal("入参本身不该被改写")
	}
	got, _ := s.ListCollabRoles(context.Background(), &pb.ListCollabRolesRequest{})
	if strings.TrimSpace(got.GetRoles()[0].GetInstructions()) != "缩进是有意义的" {
		t.Fatalf("instructions 应原样保存(含缩进),实际 %q", got.GetRoles()[0].GetInstructions())
	}
}

// ⑤ 优先级:本次显式给的值盖过角色;角色只填空。
func TestResolveDelegateRolePriority(t *testing.T) {
	s := newTestServer(t)
	if _, err := s.SetCollabRole(context.Background(), &pb.SetCollabRoleRequest{
		Role: &pb.CollabRole{Id: "r1", AgentId: "agent-from-role", Model: "m-role", Effort: "e-role", Workspace: "/ws-role"},
	}); err != nil {
		t.Fatal(err)
	}

	// 全空 → 全部取角色。
	r1, err := s.resolveDelegateWithRole(&pb.DelegateSubTaskRequest{RoleId: "r1", Task: "T"})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if r1.agentID != "agent-from-role" || r1.model != "m-role" || r1.effort != "e-role" || r1.workspace != "/ws-role" {
		t.Fatalf("角色值应填满空字段: %+v", r1)
	}

	// 显式给的必须赢(否则父模型没法对单个任务临时换模型)。
	r2, err := s.resolveDelegateWithRole(&pb.DelegateSubTaskRequest{
		RoleId: "r1", Task: "T", AgentId: "agent-explicit", Model: "m-explicit", Workspace: "/ws-explicit",
	})
	if err != nil {
		t.Fatal(err)
	}
	if r2.agentID != "agent-explicit" || r2.model != "m-explicit" || r2.workspace != "/ws-explicit" {
		t.Fatalf("显式值应优先: %+v", r2)
	}
	if r2.effort != "e-role" { // 没显式给的仍然取角色
		t.Fatalf("未显式给的字段应仍取角色值,实际 %q", r2.effort)
	}

	// 不带 role_id = 纯透传(与加本功能之前完全等价)。
	r3, err := s.resolveDelegateWithRole(&pb.DelegateSubTaskRequest{AgentId: "a", Task: "raw task"})
	if err != nil {
		t.Fatal(err)
	}
	if r3.task != "raw task" || r3.agentID != "a" {
		t.Fatalf("无 role_id 时应原样透传: %+v", r3)
	}
}

// ⑥ 未知 id 必须报错并列出可用项(让父模型能自纠,而不是反复重试)。
func TestResolveDelegateUnknownRole(t *testing.T) {
	s := newTestServer(t)

	// 一个都没配:要说"还没配过",不能给个空列表糊弄。
	_, err := s.resolveDelegateWithRole(&pb.DelegateSubTaskRequest{RoleId: "nope"})
	if err == nil || !strings.Contains(err.Error(), "还没有配置过任何协同角色") {
		t.Fatalf("期望'还没配过'的提示,实际: %v", err)
	}

	if _, err := s.SetCollabRole(context.Background(), &pb.SetCollabRoleRequest{Role: &pb.CollabRole{Id: "reviewer"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetCollabRole(context.Background(), &pb.SetCollabRoleRequest{Role: &pb.CollabRole{Id: "tester"}}); err != nil {
		t.Fatal(err)
	}
	_, err = s.resolveDelegateWithRole(&pb.DelegateSubTaskRequest{RoleId: "nope"})
	if err == nil {
		t.Fatal("未知 id 应报错")
	}
	msg := err.Error()
	if !strings.Contains(msg, "reviewer") || !strings.Contains(msg, "tester") {
		t.Fatalf("错误信息应列出可用 id 供自纠,实际: %v", msg)
	}
}

// ⑦ 人设拼接规则:全空不加任何块(不为零信息付 token),有值才注入。
func TestCollabRoleTaskComposition(t *testing.T) {
	base := "审查这个函数"

	// 空角色 → 一个字都不该多。
	if got := collabRoleTask(store.CollabRole{ID: "bare"}, base); got != base {
		t.Fatalf("无 description/instructions 时不该注入任何块,实际: %q", got)
	}
	// 有值 → 含 id 与两个字段,且原任务在最后。
	got := collabRoleTask(store.CollabRole{
		ID: "r", Name: "审查者", Description: "只找安全问题", Instructions: "列行号",
	}, base)
	for _, want := range []string{"审查者", "id=r", "只找安全问题", "列行号", base} {
		if !strings.Contains(got, want) {
			t.Fatalf("注入块应含 %q,实际: %q", want, got)
		}
	}
	if strings.Index(got, "职责") > strings.Index(got, base) {
		t.Fatalf("人设必须在本次任务之前: %q", got)
	}
	// name 为空 → 回落 id,不伪造"未命名"之类的人造默认值。
	got2 := collabRoleTask(store.CollabRole{ID: "anon", Description: "d"}, base)
	if !strings.Contains(got2, "anon") || strings.Contains(got2, "未命名") {
		t.Fatalf("name 为空应回落 id,实际: %q", got2)
	}
}

// ---- 辅助 ----

func idsOf(resp *pb.ListCollabRolesResponse) []string {
	out := make([]string, 0, len(resp.GetRoles()))
	for _, r := range resp.GetRoles() {
		out = append(out, r.GetId())
	}
	return out
}
