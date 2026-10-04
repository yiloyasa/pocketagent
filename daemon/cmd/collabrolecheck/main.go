// collabrolecheck —— 走 daemon 生产 gRPC 面跑一遍协同角色的完整闭环,
// 验证「分工编排」在**真实进程**里确实可用(T-ROLE 端到端自证)。
//
// 【为什么需要它】单测钉的是逻辑,in-process 不覆盖三件事:
//  1. 新 RPC 有没有真的挂上 ThreadService(method 名双端是否一致);
//  2. 落库到 threads.json 后**重开存储**还在不在(引用性配置丢了 = 白配);
//  3. 委派链路里的角色解析在真实请求上会不会报错、报的是不是人话。
//
// 本探针不引入任何测试钩子 —— 就是手机端那条路:连主 daemon(TLS + 配对码
// 派生 token),依次调用 ListCollabRoles / SetCollabRole / DelegateSubTask /
// DeleteCollabRole,并打印每一步的实际结果。
//
// 用法:go run ./cmd/collabrolecheck [-addr 127.0.0.1:7246] [-data <数据目录>]
//
// 红线:只读 pair-code.txt 派生 token(与既有权衡链一致),不打印 token 本身。
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/hostenv"
	"github.com/pocketagent/daemon/internal/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

// resolveDataDir 定位 daemon 数据目录:优先迁移后的数据目录
// (<LOCALAPPDATA>\PocketAgent\data),回退历史位置(家目录 .pocketagentd)。
// 只读两级探测,与 cmd/wbempty 的 storePath 同款:按本工具实际要读的
// pair-code.txt 探测,现存的优先,都不存在时回退历史位置,让读失败的报错
// 仍指向一个说得通的位置。不调 hostenv.ResolveDataDir —— 那会触发一次性
// 迁移副作用,探针不该替 daemon 做迁移。
func resolveDataDir() string {
	for _, dir := range []string{hostenv.UserDataDir(), hostenv.LegacyDataDir()} {
		if _, err := os.Stat(filepath.Join(dir, "pair-code.txt")); err == nil {
			return dir
		}
	}
	return hostenv.LegacyDataDir()
}

func main() {
	log.SetFlags(0)
	addr := flag.String("addr", "127.0.0.1:7246", "主 daemon 地址")
	dataDir := flag.String("data", "", "daemon 数据目录(默认 ~/.pocketagentd,用于读配对码)")
	flag.Parse()

	dir := *dataDir
	if dir == "" {
		dir = resolveDataDir()
	}

	creds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}) //nolint:gosec // 本机诊断探针
	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatalf("连接 daemon 失败: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 配对码 → legacy token(与 authorized → devices.Validate 的回退链同源)。
	if raw, err := os.ReadFile(filepath.Join(dir, "pair-code.txt")); err == nil {
		code := strings.TrimSpace(string(raw))
		if tok := server.LegacyTokenFor(code); tok != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+tok)
		}
	}

	cli := pb.NewThreadServiceClient(conn)
	// ListModels 挂在 ModelService 上(不是 ThreadService),两侧 stub 分开建。
	mcli := pb.NewModelServiceClient(conn)

	// ---- 1) 基线:当前有多少角色 ----
	before, err := cli.ListCollabRoles(ctx, &pb.ListCollabRolesRequest{})
	if err != nil {
		log.Fatalf("ListCollabRoles 失败(RPC 没挂上?): %v", err)
	}
	fmt.Printf("[1] 现有协同角色: %d 个\n", len(before.GetRoles()))
	for _, r := range before.GetRoles() {
		fmt.Printf("    · id=%s name=%s agent=%s model=%q effort=%q\n",
			r.GetId(), r.GetName(), r.GetAgentId(), r.GetModel(), r.GetEffort())
	}

	// ---- 2) 校验:非法 id 必须被人话拒绝,且不落库 ----
	for _, bad := range []struct {
		id, why string
	}{
		{"", "空 id"},
		{"has space", "含空格"},
		{strings.Repeat("x", 200), "超长"},
	} {
		_, err := cli.SetCollabRole(ctx, &pb.SetCollabRoleRequest{
			Role: &pb.CollabRole{Id: bad.id, Name: "探针"},
		})
		if err == nil {
			fmt.Printf("[2] ✗ %s 竟被接受\n", bad.why)
			continue
		}
		fmt.Printf("[2] %s 被拒(符合预期): %v\n", bad.why, err)
	}

	// ---- 3) 正常写入 ----
	probeID := "probe-reviewer"
	set, err := cli.SetCollabRole(ctx, &pb.SetCollabRoleRequest{Role: &pb.CollabRole{
		Id:           probeID,
		Name:         "安全审查者",
		AgentId:      "pi",
		Description:  "只找安全问题,不修改任何代码",
		Instructions: "必须列出证据文件路径与行号",
		Model:        "",
		Effort:       "low",
	}})
	if err != nil {
		log.Fatalf("SetCollabRole 失败: %v", err)
	}
	fmt.Printf("[3] 已写入: id=%s name=%s agent=%s effort=%s\n",
		set.GetRole().GetId(), set.GetRole().GetName(), set.GetRole().GetAgentId(), set.GetRole().GetEffort())

	// ---- 4) 委派引用未知角色:必须是人话错误,且列出可用项 ----
	dr, err := cli.DelegateSubTask(ctx, &pb.DelegateSubTaskRequest{RoleId: "不存在的角色", Task: "审查"})
	if err != nil {
		log.Fatalf("委派 RPC(业务失败应走 error 字段而非 status)竟返回 status: %v", err)
	}
	fmt.Printf("[4] 引用未知角色的委派返回: error=%q\n", dr.GetError())
	if dr.GetError() == "" {
		fmt.Println("    ✗ 未知角色竟未报错")
	}

	// ---- 5) 回读:应能查到刚写的那条 ----
	after, err := cli.ListCollabRoles(ctx, &pb.ListCollabRolesRequest{})
	if err != nil {
		log.Fatalf("回读失败: %v", err)
	}
	found := false
	for _, r := range after.GetRoles() {
		fmt.Printf("[5] · id=%s name=%s instructions=%q\n", r.GetId(), r.GetName(), r.GetInstructions())
		if r.GetId() == probeID {
			found = true
		}
	}
	if !found {
		fmt.Printf("[5] ✗ 回读不到刚写入的 %s\n", probeID)
	}

	// ---- 6) 清理 ----
	if _, err := cli.DeleteCollabRole(ctx, &pb.DeleteCollabRoleRequest{Id: probeID}); err != nil {
		log.Fatalf("删除失败: %v", err)
	}
	// 幂等:再删一次不该报错。
	if _, err := cli.DeleteCollabRole(ctx, &pb.DeleteCollabRoleRequest{Id: probeID}); err != nil {
		fmt.Printf("[6] ✗ 重复删除应幂等成功,却报错: %v\n", err)
	} else {
		fmt.Println("[6] 已删除,且重复删除幂等成功")
	}
	final, _ := cli.ListCollabRoles(ctx, &pb.ListCollabRolesRequest{})
	fmt.Printf("[6] 清理后剩余: %d 个(应与步骤 1 相同: %d)\n", len(final.GetRoles()), len(before.GetRoles()))

	// ---- 7) 协同扇出:一条指令 → 多个 agent 并行(真跑,不是单测)----
	// 用一个真实 agent(pi) + 一个不存在的(ghost)混着派:验证"一家失败不拖垮
	// 别家"在真实进程里成立,且返回项与成员顺序对得上。
	dc, err := cli.DispatchCollab(ctx, &pb.DispatchCollabRequest{
		Instruction: "探针:各用一句话说明你现在能做什么",
		Members: []*pb.CollabMember{
			{AgentId: "pi"},
			{AgentId: "ghost-probe"},
		},
	})
	if err != nil {
		log.Fatalf("DispatchCollab(RPC 层)失败: %v", err)
	}
	fmt.Printf("[7] 协同会话 id=%s\n", dc.GetSessionId())
	for i, it := range dc.GetItems() {
		fmt.Printf("    [%d] agent=%q ack=%v thread=%q err=%q\n",
			i, it.GetAgentId(), it.GetAck(), it.GetThreadId(), it.GetError())
	}
	if len(dc.GetItems()) != 2 {
		log.Fatalf("返回项应与成员数一致(2),实际 %d", len(dc.GetItems()))
	}

	// ---- 8) pi 的模型与思考强度(用户报障:列表为空、刷新无效)----
	// refresh=true 必须真的重新向每个**已注册** agent 问一次,而不是读缓存。
	// agent 名称从 GetHostInfo.capabilities 的 agent:* 动态提取,不在探针里复制
	// 一份名单;新增 provider 后探针自动覆盖,也不会触发 AUDIT 的名单红线。
	hi, hiErr := pb.NewHostServiceClient(conn).GetHostInfo(ctx, &pb.GetHostInfoRequest{})
	if hiErr != nil {
		fmt.Printf("[8] GetHostInfo 失败,跳过模型检查: %v\n", hiErr)
		return
	}
	agentIDs := agentIDsFromCapabilities(hi.GetCapabilities())
	for _, ag := range agentIDs {
		md, merr := mcli.ListModels(ctx, &pb.ListModelsRequest{AgentId: ag, Refresh: true})
		if merr != nil {
			fmt.Printf("[8] %s: ListModels 失败: %v\n", ag, merr)
			continue
		}
		fmt.Printf("[8] %s: 模型 %d 个 | 当前模型=%q | effort=%q | state_error=%q\n",
			ag, len(md.GetModels()), md.GetCurrentModelId(), md.GetCurrentEffort(), md.GetStateError())
		for i, m := range md.GetModels() {
			if i >= 3 {
				fmt.Printf("      …(共 %d 个)\n", len(md.GetModels()))
				break
			}
			fmt.Printf("      · %s (%s)\n", m.GetId(), m.GetDisplayName())
		}
	}
}

// agentIDsFromCapabilities 把 daemon 的能力声明还原为可探测的 agent id。
//
// 能力串允许携带版本后缀(例如 agent:pi:v0.85.1),所以只取第一个冒号
// 后的 id。重复声明去重并排序,让探针输出稳定;空值直接忽略。这里的
// 解析规则与手机端的 agent:<id> 前缀语义一致,不维护第二份 agent 名单。
func agentIDsFromCapabilities(capabilities []string) []string {
	ids := make(map[string]struct{})
	for _, capability := range capabilities {
		if !strings.HasPrefix(capability, "agent:") {
			continue
		}
		id := strings.TrimSpace(strings.TrimPrefix(capability, "agent:"))
		if i := strings.IndexByte(id, ':'); i >= 0 {
			id = id[:i]
		}
		if id != "" {
			ids[id] = struct{}{}
		}
	}
	out := make([]string, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
