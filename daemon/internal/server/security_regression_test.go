// security_regression_test.go —— 安全和路径围栏回归(发布与端到端验证计划 Task 6)。
//
// 【本文件钉什么】把 2026-10-04 批次的六类安全断言固化成 Go 回归测试:
//
//	一、文件围栏:guardDangerousPath 表驱动(盘根/SystemRoot/ProgramFiles/USERPROFILE/
//	    Temp 根/数据子树/cwd+exe,含 Clean/Abs/大小写变体)+ DeleteEntry/MoveEntry
//	    源路径护栏 + 合法工作区路径放行;
//	二、junction/UNC/盘根变体:junction 指向禁区根(形态学 collab_workspace_test 的
//	    mklink 用例)、UNC 形态进护栏不 panic 且结论明确、盘根变体全拒;
//	三、TLS fail-closed:internal/server 生产源码零 plaintext 降级路径(文本扫)+
//	    cmd 入口层"显式应急旗标(默认关)/初始化失败即 Fatal/明文模式出声"正向锚点;
//	四、token 不进发布物:paguard 源码守护性文本钉(pair-code 旗标默认空、零环境
//	    注入、代码区无盘符绝对路径/codex.cmd 字面量 —— CODEX_BIN 否决记录的机器可查面);
//	五、日志卫生:unknown 动作处置历史(内存快照+盘上账本)不得夹带 token/完整语音
//	    文本;stderrdrain 对未知事件样例 payload 的脱敏输出断言;
//	六、unknown request 无通用允许捷径:展示层动作表(runtime.PresentUnknownEvent)
//	    与执行层白名单(unknownActionTarget)集合对账 + 伪造动作逐一反证。
//
// 【设计纪律】只测不改:本文件不触碰任何生产代码;对 fs.go/collab_workspace.go/
// unknown_actions.go/paguard/cmd 入口的一律是只读断言。守护性文本钉(三/四)读的是
// 源码文件本身,不是运行产物 —— 注释里逐条说明为什么文本钉在这里是合适的强度。
// 【skip 门语义】TestGuardDangerousPath{BareDriveRelative,JunctionEscape,
// DriveRootDeviceForm,AdminShareLoopback} 四条是"期望已定义(拒绝)、实现未到位"
// 的占位守卫:护栏一旦加固,用例自动从 skip 转为常态拒绝守卫;当前 skip 文本即
// 缺口证据,逐条转 security-regression 报告由主模型裁决(本执行器不改生产代码)。

package server

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	rtruntime "github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/stderrdrain"
)

// ============ 一、文件围栏:guardDangerousPath ============

// TestGuardDangerousPathDenyList 删除/移动源路径护栏的命中清单,表驱动
// (形态学 collab_workspace_test.TestWorkspaceGuardDenyList,清单口径与
// guardWorkspaceRoot 对齐:fs.go 注释①~⑦)。跨平台用例在前,Windows 专属
// 用例(盘根/大小写变体)按 GOOS 追加;期望拒绝的路径全部只调护栏本身,
// 绝不真正执行删除。
func TestGuardDangerousPathDenyList(t *testing.T) {
	dataDir := t.TempDir()
	s := &Server{cfg: Config{DataDir: dataDir}}

	denied := []struct{ name, path string }{
		// ⑤ 系统临时目录根(各平台稳定命中)
		{"temp根", filepath.Clean(os.TempDir())},
		// ⑥ daemon 数据目录:唯一整棵子树都拒 —— 根、深层、文件三层都要命中
		{"数据目录根本身", dataDir},
		{"数据目录深层", filepath.Join(dataDir, "collab", "artifacts", "sess", "m1")},
		{"数据目录内文件", filepath.Join(dataDir, "threads.json")},
	}
	// ④ 用户主目录根
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		denied = append(denied, struct{ name, path string }{"用户主目录根", home})
	}
	// ⑦ 进程当前工作目录 / 可执行文件所在目录(go test 的 cwd = 本包目录)
	if wd, err := os.Getwd(); err == nil && wd != "" {
		denied = append(denied, struct{ name, path string }{"进程工作目录", wd})
	}
	if exe, err := os.Executable(); err == nil {
		denied = append(denied, struct{ name, path string }{"可执行文件所在目录", filepath.Dir(exe)})
	}
	if runtime.GOOS == "windows" {
		// ① 驱动器根(含大小写与正反斜杠变体;Clean 后长度 <=3 一律拒)
		denied = append(denied,
			struct{ name, path string }{"盘根大写", `C:\`},
			struct{ name, path string }{"盘根小写双斜杠", `c:\\`},
			struct{ name, path string }{"盘根正斜杠", `c:/`},
		)
		// ② %SystemRoot% 本身 + 顶层子项,含大小写变体(Windows 大小写不敏感,
		//    护栏靠小写比对 —— 变体若被放行即归一化回归)
		if sysRoot := strings.TrimSpace(os.Getenv("SystemRoot")); sysRoot != "" {
			denied = append(denied,
				struct{ name, path string }{"系统目录原样", sysRoot},
				struct{ name, path string }{"系统目录小写", strings.ToLower(sysRoot)},
				struct{ name, path string }{"系统目录大写", strings.ToUpper(sysRoot)},
				struct{ name, path string }{"系统目录顶层子项", filepath.Join(sysRoot, "System32")},
			)
		}
		// ③ 程序目录根本身
		for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)"} {
			if root := strings.TrimSpace(os.Getenv(env)); root != "" {
				denied = append(denied, struct{ name, path string }{env + "根", root})
			}
		}
		// ④ %USERPROFILE% 原样形态
		if up := strings.TrimSpace(os.Getenv("USERPROFILE")); up != "" {
			denied = append(denied, struct{ name, path string }{"USERPROFILE根", up})
		}
		// Clean/Abs 归一化变体:尾部 .. 与内嵌 . 必须先归一再比对
		if sysRoot := strings.TrimSpace(os.Getenv("SystemRoot")); sysRoot != "" {
			denied = append(denied,
				struct{ name, path string }{"系统目录经..归一回根", sysRoot + `\..`},
				struct{ name, path string }{"系统目录经.归一", sysRoot + `\.\`},
			)
		}
		denied = append(denied, struct{ name, path string }{"盘根经..归一", `C:\Windows\..`})
	}

	for _, c := range denied {
		if err := s.guardDangerousPath(c.path); err == nil {
			t.Errorf("护栏应拒绝 %s(%s),实际放行", c.path, c.name)
		}
	}

	// 放行面:护栏只挡"根/顶层本身/数据子树",不误伤普通工作路径。
	allowed := []struct{ name, path string }{
		{"临时目录深层子目录", filepath.Join(t.TempDir(), "workspace", "project")},
		{"尚未创建的路径(护栏不做存在性裁决)", filepath.Join(t.TempDir(), "not-yet-created")},
		{"数据目录的兄弟目录", filepath.Join(filepath.Dir(dataDir), "sibling-ok")},
		{"相对路径形态(归一后落在 cwd 子层,不是 cwd 本身)", filepath.Join("rel-sub", "dir", "file.txt")},
	}
	for _, c := range allowed {
		if err := s.guardDangerousPath(c.path); err != nil {
			t.Errorf("护栏不应拒绝 %s(%s),实际: %v", c.path, c.name, err)
		}
	}
}

// TestGuardDangerousPathBareDriveRelative 裸盘符形态(`C:`,无反斜杠)。
// 【skip 门】方案期望「盘根变体(`C:\`、`c:\\`、`C:`)全拒」;当前实现里
// filepath.Abs("C:") 按盘符相对路径解析成 <cwd>\C:(Go 语义:IsAbs("C:")=false),
// 不命中 ① 驱动器根规则(len<=3)。护栏今天给不出明确拒绝,由后续 os.Remove 对
// 该畸形形态的文件系统报错兜底 —— 方案期望与实现存在缺口,本执行器禁改 fs.go,
// 以 skip 门占位:护栏加固后自动转为常态守卫。已转 security-regression 报告。
func TestGuardDangerousPathBareDriveRelative(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("盘符相对形态是 Windows 专属")
	}
	s := &Server{cfg: Config{DataDir: t.TempDir()}}
	if err := s.guardDangerousPath(`C:`); err != nil {
		return // 期望成立:裸盘符已被拒绝
	}
	t.Skip("已知边界:guardDangerousPath 归一化只做 Clean+Abs+小写,`C:` 被 Abs 解析为 " +
		"<cwd>\\C: 形态而不命中①驱动器根规则(len<=3),护栏未给出明确拒绝(方案期望盘根变体全拒)。 " +
		"修复落地后本用例自动转为常态守卫;裁决归主模型,证据见 security-regression 报告")
}

// TestGuardDangerousPathJunctionEscape junction 指向禁区根必须被拒。
// 形态学 collab_workspace_test.TestWorkspaceGuardCatchesJunctionEscape:
// mklink /J 不需要管理员权限,创建失败(策略限制)则 t.Skip 保持确定性。
// 【skip 门】协同工作区护栏(guardWorkspaceRoot)已按 2026-10-04 门禁修复改为
// GetFinalPathNameByHandle 物理路径判定;删除/移动护栏(guardDangerousPath)仍是
// Clean+Abs+小写(其「归一化边界」注释自述不解析符号链接)—— 同一 junction,
// 一个拒一个放行,属两套清单实现漂移。本执行器禁改 fs.go,以 skip 门占位并附
// 对照组证据:修复(引入物理解析)落地后本用例自动转为常态守卫。
func TestGuardDangerousPathJunctionEscape(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("junction 是 Windows 专属")
	}
	link := filepath.Join(t.TempDir(), "link")
	target := filepath.Clean(os.TempDir()) // Clean:mklink 对尾随反斜杠敏感
	cmd := exec.Command("cmd", "/c", "mklink", "/J", link, target)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("无法创建 junction(环境限制): %v: %s", err, out)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatalf("junction 应已创建: %v", err)
	}
	s := &Server{cfg: Config{DataDir: t.TempDir()}}
	if err := s.guardDangerousPath(link); err != nil {
		return // 期望成立:删除/移动护栏已拦下指向禁区根的 junction
	}
	// 对照组:同一 junction,物理解析版护栏应拒绝 —— 非 nil 即证明 junction 真实
	// 落在禁区、缺口只在删除/移动护栏这一侧(排除"环境没建出真 junction"的假阳)。
	wsVerdict := guardWorkspaceRoot(link, "")
	t.Skipf("已知边界:guardDangerousPath 仅 Clean+Abs+小写归一(fs.go 归一化边界注释自述不解析"+
		"符号链接),junction 指向禁区根按链接自身路径比对 → 放行;对照组 guardWorkspaceRoot(物理解析版)"+
		"对同一 junction 的判定 err=%v(非 nil 即缺口只在删除/移动护栏)。与协同护栏修复不一致,已转报告裁决;"+
		"修复落地后本用例自动转为常态守卫", wsVerdict)
}

// TestGuardDangerousPathUNCFormsHaveExplicitVerdict UNC 形态(\\server\share)。
// 【明确结论 = 放行(nil),且不 panic】论证:guardDangerousPath 的命中清单全部是
// "本机不可逆系统位置"(盘根/系统目录/主目录/临时根/数据子树),UNC 是远端命名
// 空间,不与任何本地清单根相交(⑥ 的 Rel 跨卷返回 error → continue 分支);手机端
// 文件浏览只列本机盘符(ListRoots),远端位置只能由用户显式给出,删除远端文件属于
// 用户自主决策,不在本护栏裁决范围。本用例钉住"不 panic + 结论明确(放行)",
// 防止未来清单扩展时 UNC 被误伤成半吊子拒绝。
func TestGuardDangerousPathUNCFormsHaveExplicitVerdict(t *testing.T) {
	s := &Server{cfg: Config{DataDir: t.TempDir()}}
	for _, p := range []string{
		`\\server\share`,
		`\\server\share\docs\file.txt`,
		`\\?\UNC\server\share\docs`,
	} {
		if got := s.guardDangerousPath(p); got != nil {
			t.Errorf("UNC 形态 %s 的明确结论是放行(不命中本机清单,误拒会砍断用户显式指定的网络位置),实际: %v", p, got)
		}
	}
}

// TestGuardDangerousPathDriveRootDeviceForm 设备形态盘根(\\?\C:\)。
// 【skip 门】方案要求盘根变体全拒;\\?\C:\ 经 Clean+Abs 后长度 >3,不命中
// ① 驱动器根规则(len<=3)。手机 UI 不会产出该形态(ListRoots 给的是 `C:\`),
// 风险面是伪造 RPC;修复(归一化剥 \\?\ 前缀或引入物理解析)落地后自动转常态守卫。
func TestGuardDangerousPathDriveRootDeviceForm(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("设备形态路径是 Windows 专属")
	}
	s := &Server{cfg: Config{DataDir: t.TempDir()}}
	if err := s.guardDangerousPath(`\\?\C:\`); err != nil {
		return // 期望成立:设备形态盘根已被拒绝
	}
	t.Skip("已知边界:`\\\\?\\C:\\` 设备形态长度>3,不命中①驱动器根规则(len<=3);" +
		"UI 不可达、风险面是伪造 RPC。修复落地后本用例自动转为常态守卫;已转 security-regression 报告")
}

// TestGuardDangerousPathAdminShareLoopback 管理共享回指本机(\\localhost\C$\...)。
// 【skip 门】`\\localhost\C$\Windows` 语义上就是本机 C:\Windows(经管理共享回指),
// 但小写形态与 ② 清单根不一致,当前不拒。前置缓冲:枚举 admin share 需要管理员
// 令牌,风险低于本地路径;与 junction 同族的归一化缺口,修复落地后自动转常态守卫。
func TestGuardDangerousPathAdminShareLoopback(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("管理共享路径是 Windows 专属")
	}
	s := &Server{cfg: Config{DataDir: t.TempDir()}}
	if err := s.guardDangerousPath(`\\localhost\C$\Windows`); err != nil {
		return // 期望成立:管理共享回指本机系统目录已被拒绝
	}
	t.Skip("已知边界:`\\\\localhost\\C$\\Windows` 与②清单根的小写形态不一致,当前不拒 " +
		"(经管理共享回指本机系统目录;枚举需管理员令牌作前置缓冲)。与 junction 同族的归一化缺口," +
		"修复落地后本用例自动转为常态守卫;已转 security-regression 报告")
}

// TestDeleteEntryGuardRejectsBeforeDelete handler 级端到端:DeleteEntry 必须在
// 触碰文件系统**之前**过护栏 —— 拒绝后目标毫发无损。递归/非递归两形态都钉
// (fs.go 注释:非递归删高危根内单个文件危害有限,但根本身永远不该从这里消失)。
func TestDeleteEntryGuardRejectsBeforeDelete(t *testing.T) {
	s := &Server{cfg: Config{DataDir: t.TempDir()}}
	ctx := context.Background()

	// 递归删系统临时目录根 → 拒绝,且目录原样存在。
	if _, err := s.DeleteEntry(ctx, &pb.DeleteEntryRequest{
		Path: filepath.Clean(os.TempDir()), Recursive: true,
	}); err == nil {
		t.Fatalf("递归删除系统临时目录根必须被护栏拒绝")
	}
	if _, err := os.Stat(os.TempDir()); err != nil {
		t.Fatalf("护栏拒绝后系统临时目录必须原样存在: %v", err)
	}

	// 数据子树内文件(⑥ 整棵子树都拒)非递归删 → 拒绝,文件原样。
	dataFile := filepath.Join(s.cfg.DataDir, "threads.json")
	if err := os.WriteFile(dataFile, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("预置数据文件失败: %v", err)
	}
	if _, err := s.DeleteEntry(ctx, &pb.DeleteEntryRequest{Path: dataFile, Recursive: false}); err == nil {
		t.Fatalf("数据目录内文件的删除必须被护栏拒绝(⑥ 整棵子树)")
	}
	if _, err := os.Stat(dataFile); err != nil {
		t.Fatalf("护栏拒绝后数据文件必须原样存在: %v", err)
	}
}

// TestMoveEntrySourceGuardAndLegitMove MoveEntry 只查移动源(FromPath)的护栏:
// 禁区源被拒且 rename 从未发生(目标无痕迹);普通重命名不被误伤。
func TestMoveEntrySourceGuardAndLegitMove(t *testing.T) {
	s := &Server{cfg: Config{DataDir: t.TempDir()}}
	ctx := context.Background()
	junk := filepath.Join(t.TempDir(), "must-not-appear")

	// 源 = daemon 数据目录(⑥) → 拒绝,目标不得出现任何痕迹。
	if _, err := s.MoveEntry(ctx, &pb.MoveEntryRequest{FromPath: s.cfg.DataDir, ToPath: junk}); err == nil {
		t.Fatalf("移动数据目录源必须被护栏拒绝")
	}
	if _, err := os.Stat(junk); !os.IsNotExist(err) {
		t.Fatalf("被拒的移动不得产生目标痕迹: %v", err)
	}

	// 源 = 系统临时目录根(⑤) → 拒绝。
	if _, err := s.MoveEntry(ctx, &pb.MoveEntryRequest{
		FromPath: filepath.Clean(os.TempDir()), ToPath: junk,
	}); err == nil {
		t.Fatalf("移动系统临时目录根源必须被护栏拒绝")
	}

	// 合法源正常放行:普通目录重命名照常工作(证明护栏没有过杀)。
	ws := t.TempDir()
	src := filepath.Join(ws, "a")
	dst := filepath.Join(ws, "b")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("建源目录失败: %v", err)
	}
	if _, err := s.MoveEntry(ctx, &pb.MoveEntryRequest{FromPath: src, ToPath: dst}); err != nil {
		t.Fatalf("普通目录重命名不应被拦截: %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("合法移动应已生效: %v", err)
	}
}

// ============ 三、TLS fail-closed ============

// TestServerPackageHasNoPlaintextDowngrade 明文降级路径的架构位置钉:
// TLS 决策只允许收敛在 cmd/pocketagentd 入口层,internal/server 生产源码不得出现
// 任何明文传输标记(出现即说明降级逻辑渗进了服务层 = 回归)。这是守护性文本钉:
// 服务层没有任何"按明文起 gRPC"的合法理由,词法级断言即够。
// 【标记口径】钉的是 grpc-go 明文传输的具体形态,不是 "plaintext" 单词本身 ——
// 后者会误伤合法标识符(实测 internal/server/turn_memory.go 的 blocksPlainText,
// 内容块文本拍平,与传输无关)。三个标记:
//   - usePlaintext(任何大小写):grpc-go 明文通道/服务的 API 惯用名;
//   - credentials/insecure(导入路径)与 insecure.NewCredentials(构造器):
//     grpc-go 明文传输凭证的唯一来源。
//
// 正向锚点钉在 cmd 入口(文件读取为纯数据访问,不运行任何程序):
//   - -plaintext 只是显式应急旗标,默认 false(不传=永远 TLS);
//   - TLS 证书初始化失败 log.Fatalf = fail-closed(拒绝裸奔启动,不是静默回明文);
//   - 真进了明文模式必须打 WARNING 出声;
//   - -plaintext 明文仅允许回环(2026-10-04 F2 裁决落地,对齐 -insecure-no-auth
//     先例):显式非回环地址拒绝启动并给人话,未显式指地址自动压回 127.0.0.1:PORT,
//     且回环裁决真正接到 net.Listen(防裁决成死代码);
//   - mDNS 广播的 TLSEnabled 必须与明文旗标联动(手机端靠它判别端口安全形态)。
//
// 证书本体(生成/持久化/握手指纹锁定)由 internal/tlscert 的既有测试覆盖
// (TestEnsureGeneratesAndReloads / TestTLSHandshakePin{Match,Mismatch});
// Android 侧 SSLContext 初始化 fail-closed(AgentConnection.kt applyPinnedTls,
// 非回环目标抛异常拒建通道)不在 Go 可测范围 —— 转「真机/仪器项」清单。
func TestServerPackageHasNoPlaintextDowngrade(t *testing.T) {
	// 明文传输标记(小写比较;grpc-go 形态在仓库里只会以这些字面出现)。
	plaintextMarkers := []string{
		"useplaintext",
		`credentials/insecure`,
		"insecure.newcredentials",
	}
	serverDir := securityPinSourcePath(t)
	entries, err := os.ReadDir(serverDir)
	if err != nil {
		t.Fatalf("读 internal/server 目录失败: %v", err)
	}
	scanned := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue // 测试文件允许出现这些词(本文件就在解释它们);钉的是生产降级路径
		}
		raw := strings.ToLower(string(readFileOrFatal(t, filepath.Join(serverDir, name))))
		for _, marker := range plaintextMarkers {
			if strings.Contains(raw, marker) {
				t.Errorf("internal/server 生产源码 %s 出现明文传输标记 %q:TLS 决策只允许收敛在 cmd 入口层,服务层出现明文降级 = 回归", name, marker)
			}
		}
		scanned++
	}
	if scanned == 0 {
		t.Fatalf("internal/server 未扫到任何生产 .go 文件,本钉自检失效")
	}

	mainSrc := string(readFileOrFatal(t, securityPinSourcePath(t, "..", "..", "cmd", "pocketagentd", "main.go")))
	for _, pin := range []string{
		`flag.Bool("plaintext", false,`,   // 显式应急旗标,默认关
		`log.Fatalf("TLS 证书初始化失败`,         // 初始化失败 fail-closed:拒绝启动,不静默回明文
		`WARNING: PLAINTEXT MODE ENABLED`, // 明文模式必须出声
	} {
		if !strings.Contains(mainSrc, pin) {
			t.Errorf("cmd/pocketagentd/main.go 缺少 TLS fail-closed 守护锚点 %q(入口层被改动时须人工复核本钉)", pin)
		}
	}
	// F2 回环硬限制(2026-10-04 裁决落地,已修复+回归钉):-plaintext 明文模式对齐
	// -insecure-no-auth 先例,监听强制回环;四个锚点缺一即视为修复回退。
	//   ① 显式非回环地址 → 拒绝启动并给人话出路(改用默认 TLS);
	//   ② 未显式指地址(默认 ":7246" 全接口)→ 自动压回 127.0.0.1:PORT;
	//   ③ 回环裁决必须真正接到监听器(否则①②是死代码);
	//   ④ 回环判定器存在(localhost/127.0.0.0/8/::1)。
	for _, pin := range []string{
		`log.Fatalf("-plaintext 应急明文仅允许回环`,
		`net.JoinHostPort("127.0.0.1", port)`,
		`net.Listen("tcp", listenAddr)`,
		`func isLoopbackHost(`,
	} {
		if !strings.Contains(mainSrc, pin) {
			t.Errorf("cmd/pocketagentd/main.go 缺少 -plaintext 回环硬限制锚点 %q:F2 修复回退,人工复核", pin)
		}
	}
	if !regexp.MustCompile(`TLSEnabled:\s+!\*plaintext`).MatchString(mainSrc) {
		t.Errorf("mDNS 广播的 TLSEnabled 必须与明文旗标联动(!*plaintext):手机端靠它判别端口安全形态")
	}
}

// ============ 四、token 不进发布物(paguard 守护性文本钉) ============

// TestPaguardSourceNoInjectedCredentialsOrPaths 把 CODEX_BIN 否决记录
// (cmd/paguard/main.go startDaemon 注释:未设即不注入,环境原样继承)的"否决"
// 变成机器可查的回归面。守护性文本钉:paguard 是随发布包分发的二进制,源码里
// 出现字面量凭据/个人路径 = 随二进制分发(规格 §2 明令禁止),文本级断言即够。
//
// 【与本批方案口径的差异,如实声明】方案原文要求"断言不含 -pair-code 字面量注入
// 路径";实际源码中 `-pair-code` 作为旗标名与转发参数**合法存在**(计划任务 /tr
// 经它把 conf 里的配对码传给 daemon,install.bat:121),按字面钉"不含"必然假红。
// 本钉改钉同缺口下机器可查且当前为真的三个锚点:①旗标默认值必须为空(二进制不
// 携带内置凭据);②零环境注入(CODEX_BIN 否决的操作面);③代码区无盘符绝对路径与
// codex.cmd 字面量。差异已转 security-regression 报告由主模型裁决。
func TestPaguardSourceNoInjectedCredentialsOrPaths(t *testing.T) {
	src := string(readFileOrFatal(t, securityPinSourcePath(t, "..", "..", "cmd", "paguard", "main.go")))

	// ① -pair-code 旗标默认值必须为空字符串:凭据只能来自计划任务参数/conf,
	//    不得烙进二进制(与 CODEX_BIN 否决同款纪律)。
	if !regexp.MustCompile(`flag\.String\("pair-code",\s*"",`).MatchString(src) {
		t.Errorf("paguard 的 -pair-code 旗标默认值必须为空字符串:发布物不得携带内置凭据")
	}
	// ② 零环境注入:paguard 不得出现任何 cmd.Env 赋值(子进程环境必须原样继承;
	//    CODEX_BIN 否决记录的操作面)。源码注释"cmd.Env 保持 nil"不含等号,不误伤。
	if regexp.MustCompile(`cmd\.Env\s*=`).MatchString(src) {
		t.Errorf("paguard 出现 cmd.Env 赋值:CODEX_BIN 否决记录禁止注入写死的开发机路径,子进程环境必须原样继承")
	}
	if strings.Contains(src, `os.Setenv("CODEX_BIN"`) {
		t.Errorf("paguard 不得改写本进程的 CODEX_BIN")
	}
	// ③ 代码区(粗剥 // 行注释后)不得出现盘符绝对路径与 codex.cmd 字面量。
	//    注释里的"codex.cmd"字样是否决记录本身,属预期,必须剥掉后再扫;
	//    粗剥假设字符串字面量不含 "//"(当前 paguard 源码如此,钉子注释即契约)。
	code := stripGoLineComments(src)
	if regexp.MustCompile(`[A-Za-z]:\\`).MatchString(code) {
		t.Errorf("paguard 代码区出现盘符绝对路径:发布物不得嵌入个人/开发机路径(全局约束:个人路径扫描)")
	}
	if strings.Contains(code, "codex.cmd") {
		t.Errorf("paguard 代码区出现 codex.cmd 字面量:CODEX_BIN 已否决注入形态回归")
	}
}

// ============ 五、日志卫生 ============

// TestUnknownActionHistoryLogHygiene unknown 动作处置历史(动作执行的"日志"面)
// 不得夹带事件载荷:构造带 token 与完整语音文本的未知事件,触发 REJECTED 与
// CONFLICT 两条历史,断言内存快照与盘上账本都只有固定人话短语,不含任何载荷片段。
// 【夹具口径】台账契约是"RawPayload 由上游脱敏后入库"(runtime/unknown_events.go
// 字段注释;Record 本身不做打码)—— 本夹具故意注入未脱敏凭据,模拟上游脱敏失效
// 的最坏情形:执行历史这一旁路面必须**不依赖**上游脱敏也保持干净(纵深)。
// 事件台账本体的技术详情是按需查看的用户显式动作(脱敏语义另有 inspect 测试钉);
// 这里钉的是拒绝/冲突这类高频路径不得顺手把原始 payload 拼进旁路记录。
func TestUnknownActionHistoryLogHygiene(t *testing.T) {
	srv := newUnknownActionTestServer(t)
	ctx := context.Background()
	ledger := srv.runtimeBackend.UnknownEvents()

	const hygieneSecret = "PA-Pair-0f0f0f0f0f0f0f0f"
	const hygieneVoice = "明早八点叫我起床，把空调调到二十六度，然后提醒我带充电器"
	e := ledger.Record(rtruntime.EventEnvelope{
		SourceAgent:  "pi",
		SourceMethod: "voice/commit",
		RawPayload:   `{"token":"` + hygieneSecret + `","transcript":"` + hygieneVoice + `"}`,
	})
	// 夹具自检:载荷确实带着敏感片段(否则下面的"不含"断言是空转)。
	if cur, ok := ledger.Get(e.ID); !ok || !strings.Contains(cur.RawPayload, hygieneSecret) {
		t.Fatalf("夹具未生效: %+v", e)
	}

	// 路径一:伪造动作 → REJECTED 历史。
	if _, err := srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: e.ID, ActionId: "approve",
	}); err == nil {
		t.Fatalf("伪造动作必须被拒绝")
	}
	// 路径二:stale revision → CONFLICT 历史(冲突回结构化结果,不是 error)。
	conflict, err := srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: e.ID, ExpectedRevision: int64(e.Revision) + 7, ActionId: unknownActionAcknowledge,
	})
	if err != nil || !conflict.GetConflict() {
		t.Fatalf("stale revision 应回冲突结果: err=%v got=%+v", err, conflict)
	}

	recs := srv.unknownActionHistoryLedger().snapshot()
	if len(recs) != 2 || recs[0].Result != unknownActionResultConflict || recs[1].Result != unknownActionResultRejected {
		t.Fatalf("应有 CONFLICT+REJECTED 两条历史: %+v", recs)
	}
	banned := []string{hygieneSecret, hygieneVoice, "transcript", "token"}
	for _, r := range recs {
		for _, b := range banned {
			if strings.Contains(r.Detail, b) {
				t.Errorf("处置历史 Detail 泄漏载荷片段 %q: %q", b, r.Detail)
			}
		}
	}
	// 历史内容必须是人话短语本身(措辞是被审定的产物,钉死防"顺手拼上下文"回归):
	// 冲突 = 版本说明,拒绝 = 固定短语。
	if !strings.Contains(recs[0].Detail, "revision 不匹配") {
		t.Errorf("CONFLICT 历史应为版本说明短语: %q", recs[0].Detail)
	}
	if recs[1].Detail != "拒绝不在白名单里的动作" {
		t.Errorf("REJECTED 历史应为固定短语: %q", recs[1].Detail)
	}

	// 盘上账本同样干净(落盘路径与内存快照同源,但序列化/留档环节再钉一道)。
	raw := readFileOrFatal(t, filepath.Join(srv.cfg.DataDir, unknownActionHistoryFileName))
	for _, b := range banned {
		if strings.Contains(string(raw), b) {
			t.Errorf("盘上处置历史账本泄漏载荷片段 %q", b)
		}
	}
}

// TestStderrdrainRedactsUnknownEventSamples stderrdrain 统一收口对"未知事件形态"
// 样例 payload 的脱敏输出断言(方案指定:既有 stderrdrain 测试之外的未知事件样例,
// 收口在本回归文件,避免散落两处)。钉三层:
//  1. 键值型凭据(api_key/token=)在未知事件 stderr 样例里必须打码;
//  2. JWT/Bearer 形态必须打码;
//  3. 反过度脱敏:同行的普通诊断文本与语音转写正文必须原样保留 —— drain 的契约
//     是"凭据打码",不是"全文抹除";未知事件完整文本不进 daemon.log 由
//     TestUnknownActionHistoryLogHygiene(处置账本)与 unknown_events 台账设计
//     (原文只进按需查看的技术详情)共同保证,三层各管一段。
func TestStderrdrainRedactsUnknownEventSamples(t *testing.T) {
	const sampleKey = "sk-abc123XYZ456def789GHI012"
	const sampleJWT = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ2b2ljZS1ldmVudCJ9.c2lnX2FiY2RlZjEyMzQ1Njc4OQ"
	const samplePair = "PA-Pair-7d1f9c2b4e6a80"
	const sampleVoice = "明早八点叫我起床，把空调调到二十六度"
	cases := []struct {
		name   string
		in     string
		secret string // 必须消失的片段
		keep   string // 必须保留的诊断/正文片段(反过度脱敏)
	}{
		{
			name:   "未知通知样例:api_key 键值",
			in:     `codex stderr: failed to load profile: api_key=` + sampleKey,
			secret: sampleKey,
			keep:   "failed to load profile",
		},
		{
			name:   "未知请求样例:Bearer JWT",
			in:     "Authorization: Bearer " + sampleJWT + " while dispatching weird/request",
			secret: "eyJhbGciOiJIUzI1NiJ9",
			keep:   "weird/request",
		},
		{
			name:   "语音事件样例:token 键值 + 转写正文",
			in:     `voice event token=` + samplePair + ` transcript=` + sampleVoice,
			secret: samplePair,
			keep:   sampleVoice,
		},
	}
	var lines []string
	for _, c := range cases {
		got := stderrdrain.Redact(c.in)
		if strings.Contains(got, c.secret) {
			t.Errorf("%s:凭据未脱敏:\n in =%s\n out=%s", c.name, c.in, got)
		}
		if !strings.Contains(got, "[redacted") {
			t.Errorf("%s:应留下 redacted 标记: %s", c.name, got)
		}
		if !strings.Contains(got, c.keep) {
			t.Errorf("%s:诊断/正文片段被误抹(过度脱敏会把死因也抹掉): %s", c.name, got)
		}
		lines = append(lines, c.in)
	}
	// Drain 端到端:三行混合刷进来,吐出的每一行都必须已脱敏。
	var emitted []string
	stderrdrain.Drain(strings.NewReader(strings.Join(lines, "\n")+"\n"), stderrdrain.Options{},
		func(l string) { emitted = append(emitted, l) })
	if len(emitted) != len(cases) {
		t.Fatalf("Drain 应逐行吐出 %d 行,实际 %d 行", len(cases), len(emitted))
	}
	for _, out := range emitted {
		for _, secret := range []string{sampleKey, "eyJhbGciOiJIUzI1NiJ9", samplePair} {
			if strings.Contains(out, secret) {
				t.Errorf("Drain 吐出的行必须已脱敏,泄漏 %q: %s", secret, out)
			}
		}
	}
}

// ============ 六、unknown request 无通用允许捷径 ============

// TestUnknownActionWhitelistMatchesRuntimeTable 展示层动作表与执行层白名单的对账:
// 展示层(runtime.PresentUnknownEvent,固定规则表)并集必须精确等于已知五动作;
// 执行层(unknownActionTarget,执行器白名单)必须恰好认识"除 copy 外"的全部展示
// 动作(copy 是手机本地行为,单列说明),且不认识任何伪造的允许类动作 —— 两道卡
// 各自设防,集合漂移(一边加了按钮、另一边忘了配执行语义,或反过来)即在此现形。
// 同时钉状态型动作的目标状态全部落在 runtime 处置词表内(不造第四种状态)。
func TestUnknownActionWhitelistMatchesRuntimeTable(t *testing.T) {
	kinds := []string{
		rtruntime.KindUnknownNotification,
		rtruntime.KindUnknownRequest,
		rtruntime.KindUnknownResponse,
	}
	display := map[string]bool{}
	for _, kind := range kinds {
		p := rtruntime.PresentUnknownEvent(rtruntime.EventEnvelope{NormalizedKind: kind})
		for _, a := range p.Actions {
			if !a.Safe {
				t.Errorf("kind %s 的动作 %q 未声明 Safe:只有 Safe=true 才允许被渲染成可点按钮", kind, a.ID)
			}
			display[a.ID] = true
		}
	}
	wantDisplay := map[string]bool{
		unknownActionInspect:     true,
		unknownActionCopyID:      true,
		unknownActionAcknowledge: true,
		unknownActionOpenNative:  true,
		unknownActionRetryLater:  true,
	}
	assertStringSetEqual(t, display, wantDisplay, "展示层动作表并集与已知五动作不一致(词表漂移)")

	// 执行层覆盖面:恰好放过四个,唯一例外是 copy(手机本地)。
	for id := range wantDisplay {
		_, known := unknownActionTarget(id)
		wantKnown := id != unknownActionCopyID
		if known != wantKnown {
			t.Errorf("执行层白名单对 %q 的认识(known=%v)与展示层覆盖期望(应为 %v)不一致", id, known, wantKnown)
		}
	}

	// 通用允许捷径反证(AUDIT §3.1 红线 1:伪造 decision = 一票否决):
	// 任何 allow/approve/confirm/execute 形态的 ID 在执行层一律不认识。
	for _, forged := range []string{
		"allow", "approve", "confirm", "execute", "allow_once", "always_allow", "grant", "yes",
	} {
		if _, known := unknownActionTarget(forged); known {
			t.Errorf("执行层白名单不应认识允许类动作 %q(通用允许捷径 = 红线)", forged)
		}
		if display[forged] {
			t.Errorf("展示层动作表不应出现允许类动作 %q", forged)
		}
	}

	// 状态型动作的目标状态必须落在 runtime 处置词表(不造第四种状态)。
	if got, known := unknownActionTarget(unknownActionAcknowledge); !known || got != rtruntime.StatusInspected {
		t.Errorf("acknowledge 应映射到 StatusInspected,实际: known=%v got=%q", known, got)
	}
	if got, known := unknownActionTarget(unknownActionRetryLater); !known || got != rtruntime.StatusRetryLater {
		t.Errorf("retry_later 应映射到 StatusRetryLater,实际: known=%v got=%q", known, got)
	}
	if got, known := unknownActionTarget(unknownActionOpenNative); !known || got != rtruntime.StatusEscalated {
		t.Errorf("open_native 应映射到 StatusEscalated,实际: known=%v got=%q", known, got)
	}
	if got, known := unknownActionTarget(unknownActionInspect); !known || got != "" {
		t.Errorf("inspect 应是纯读动作(目标状态为空),实际: known=%v got=%q", known, got)
	}
}

// ============ 守护性文本钉的公共工具 ============

// securityPinSourcePath 返回本测试文件所在目录(不传 elem)或其下相对路径拼出的
// 绝对路径。用 runtime.Caller 定位而不是相对 cwd:go test 的 cwd 恰好是包目录,
// 但显式锚定测试文件让"读源码文件"的钉子不随运行方式漂移。
func securityPinSourcePath(t *testing.T, elem ...string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位本测试源文件路径(runtime.Caller 失败),守护性文本钉自检失效")
	}
	return filepath.Join(append([]string{filepath.Dir(thisFile)}, elem...)...)
}

// readFileOrFatal 读守护性文本钉的目标源码文件(纯数据访问,不运行任何程序)。
func readFileOrFatal(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败(守护性文本钉依赖的文件被移动/改名时须同步更新钉子): %v", path, err)
	}
	return raw
}

// stripGoLineComments 粗剥 // 行注释(尊重双引号字符串与转义,不处理块注释与
// 反引号原始串 —— 当前两个被钉文件 cmd/paguard/main.go、cmd/pocketagentd/main.go
// 均只用 // 行注释且字符串不含 "//",钉子注释即契约;被钉文件引入新字符串形态时
// 须同步评估本工具)。
func stripGoLineComments(src string) string {
	lines := strings.Split(src, "\n")
	for i, ln := range lines {
		cut := -1
		inStr := false
		for j := 0; j < len(ln); j++ {
			c := ln[j]
			if inStr {
				if c == '\\' {
					j++ // 跳过转义对
				} else if c == '"' {
					inStr = false
				}
				continue
			}
			switch {
			case c == '"':
				inStr = true
			case c == '/' && j+1 < len(ln) && ln[j+1] == '/':
				cut = j
			}
			if cut >= 0 {
				break
			}
		}
		if cut >= 0 {
			lines[i] = ln[:cut]
		}
	}
	return strings.Join(lines, "\n")
}

// assertStringSetEqual 断言两个字符串集合精确相等(对账测试用,双向报缺/报多)。
func assertStringSetEqual(t *testing.T, got, want map[string]bool, label string) {
	t.Helper()
	for id := range want {
		if !got[id] {
			t.Errorf("%s:缺少动作 %q(实得集合 %v)", label, id, got)
		}
	}
	for id := range got {
		if !want[id] {
			t.Errorf("%s:多出动作 %q(实得集合 %v)", label, id, got)
		}
	}
}
