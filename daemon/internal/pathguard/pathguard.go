// pathguard —— 共享路径护栏(2026-10-04 F3 收口)。
//
// 【为什么抽包】fs.go 的 guardDangerousPath(删除/移动护栏)与 collab_workspace.go
// 的 guardWorkspaceRoot(协同工作区护栏)各自维护同一份"不可逆位置"命中清单,
// 归一化口径还漂移过:删除侧只做 Clean+Abs+小写,工作区侧先改成
// GetFinalPathNameByHandle 物理锚点 —— 同一个 junction 一个拒一个放行
// (2026-10-04 审查缺口 G1)。本包把「清单判定」收口成单份实现:七项判定 +
// 全锚点物理解析 + 裸盘符/设备形态/管理共享回指的字符串层归一
// (缺口 G1-G4 的修复落点),两个调用方薄委托,只保留各自的面向文案与哨兵错误。
//
// 【判定模型】GuardDeny 对候选路径先取「物理最终路径」为主锚点
// (GetFinalPathNameByHandle:junction/符号链接/8.3 短名/大小写一次归一;
// 打不开句柄则退回归一化原路径 —— 少解析只会退化为原始形态比对,方向安全,
// 禁区根本身总能被打开,不会因此误放行),再叠加附加锚点(设备前缀剥除、
// 管理共享回指别名),逐锚点跑同一份清单,任一锚点命中即拒(fail-closed)。
//
// 【文案边界】本包只回答"拒不拒、命中哪条、用什么锚点";面向用户的错误文案
// (删除/移动视角、协同工作区视角)由调用方按 DenyKind 自拟,不把两种语义
// 塞进同一句话。
package pathguard

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pocketagent/daemon/internal/hostenv"
	"golang.org/x/sys/windows"
)

// ============ 物理路径解析(护栏的地基) ============

// 【为什么不用 filepath.EvalSymlinks】门禁实测(2026-10-04,回归钉
// TestWorkspaceGuardCatchesJunctionEscape):Windows 上 EvalSymlinks 不穿越
// NTFS junction —— junction 指向系统临时目录根,解析结果仍是 junction 自身的
// 路径,清单第⑤项形同虚设(已知行为:golang/go#21028 一族,MOUNT_POINT
// reparse point 不被按符号链接处理)。GetFinalPathNameByHandle 是 Windows 上
// "路径物理最终形态"的权威来源:junction、符号链接、8.3 短名、大小写漂移一次
// 归一 —— 本包所有"解析后路径"与"清单根"的比对都以它为准。

// fileGetNameNormalized / fileGetVolumeNameDos 是 GetFinalPathNameByHandle 的
// dwFlags 取值(MSDN:FILE_NAME_NORMALIZED=0x0、VOLUME_NAME_DOS=0x0;x/sys
// v0.47.0 未导出这两个常量,按 MSDN 原值本地定义)。
const (
	fileGetNameNormalized = 0x0
	fileGetVolumeNameDos  = 0x0
)

// finalPathFirstBufferSize 首次尝试的缓冲区宽度(WCHAR);不够时按返回值扩一次。
const finalPathFirstBufferSize = 1024

// ResolvePhysical 返回 path 的物理最终路径;ok=false = 句柄打不开(不存在/
// 权限不足/异常占用),调用方应回退用归一化原路径继续 —— 少解析只会退化为
// "原始形态比对",方向安全;禁区根自身总能被打开,不会因此误放行。
//
// 【实现】access=0(纯查询)+ FILE_FLAG_BACKUP_SEMANTICS 是打开目录句柄的
// 标准姿势,不需要任何读写权限;CreateFile 默认穿越 reparse point,句柄落在
// 物理目标上,GetFinalPathNameByHandle 据此给出最终路径。
// 【平台】仅 Windows 可编译 —— 与 hidewin/term 一致,daemon 全链本就
// Windows-only(无 build tag 先例:hidewin.go 直用 syscall 的 Windows 字段)。
func ResolvePhysical(path string) (string, bool) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return path, false
	}
	h, err := windows.CreateFile(p, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return path, false
	}
	defer windows.CloseHandle(h)

	buf := make([]uint16, finalPathFirstBufferSize)
	n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)),
		fileGetNameNormalized|fileGetVolumeNameDos)
	if n > uint32(len(buf)) {
		// 缓冲不足:返回值 = 含结尾 null 的所需宽度,扩容重试一次(上限封顶,
		// 防异常返回值拖出无界循环)。
		if n > 1<<16 {
			return path, false
		}
		buf = make([]uint16, n)
		n, err = windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)),
			fileGetNameNormalized|fileGetVolumeNameDos)
	}
	if err != nil || n == 0 || int(n) > len(buf) {
		return path, false
	}
	out := stripDevicePathPrefix(windows.UTF16ToString(buf[:n]))
	if out == "" {
		return path, false
	}
	return filepath.Clean(out), true
}

// stripDevicePathPrefix 把 GetFinalPathNameByHandle 的设备形态前缀还原为常规
// 路径:\\?\C:\a\b → C:\a\b;UNC 形态 \\?\UNC\srv\share\a → \\srv\share\a
// (VOLUME_NAME_DOS 的返回几乎总带 \\?\ 前缀,不剥会与清单根比对失败)。
func stripDevicePathPrefix(p string) string {
	if strings.HasPrefix(p, `\\?\UNC\`) {
		return `\\` + strings.TrimPrefix(p, `\\?\UNC\`)
	}
	return strings.TrimPrefix(p, `\\?\`)
}

// physicalRoot 护栏清单根的同款归一化:能打开就取物理形态,取不到(不存在/
// 打不开)退回 Clean+Abs 形态 —— 候选与根两侧同款归一,杜绝"候选侧已解析、
// 根侧还是原始形态"的比对漂移。
func physicalRoot(root string) string {
	norm, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return root
	}
	if p, ok := ResolvePhysical(norm); ok {
		return p
	}
	return norm
}

// ============ 清单式判定 ============

// DenyKind 命中清单的条目(①~⑦)。调用方按条目拟自己的面向文案;DenyNone
// 表示放行。
type DenyKind int

const (
	// DenyNone 放行(未命中任何清单条目)。
	DenyNone DenyKind = iota
	// DenyParseFail 路径归一化失败(Abs/Clean 报错):fail-closed 按拒处理。
	DenyParseFail
	// DenyDriveRoot ① 驱动器根(如 C:\)/文件系统根(Unix "/"),
	// 含裸盘符(X:)与设备形态(\\?\X:\)两族变体。
	DenyDriveRoot
	// DenySystemRoot ② %SystemRoot% 根本身(默认 C:\Windows)。
	DenySystemRoot
	// DenySystemRootChild ② %SystemRoot% 的直接子项(仅顶层一层,System32 同层)。
	DenySystemRootChild
	// DenyHomeLikeRoot ③④⑤ 程序目录/用户主目录/系统临时目录根本身。
	DenyHomeLikeRoot
	// DenyDataDir ⑥ daemon 数据目录(唯一"整棵子树都拒"的条目)。
	DenyDataDir
	// DenyProcessCwd ⑦ 进程当前工作目录本身。
	DenyProcessCwd
	// DenyExeDir ⑦ 可执行文件所在目录本身。
	DenyExeDir
)

// DenyResult GuardDeny 的判定结果。
type DenyResult struct {
	// Denied true = 命中清单,拒绝。
	Denied bool
	// Kind 命中条目;放行时为 DenyNone。【口径】不做多锚点/多条目命中计数:
	// 按「锚点顺序(主锚点→附加锚点)× 锚点内清单①→⑦顺序」取**首个**命中的
	// 条目。归一化改变落点时按归一后落点分类 —— 如 X:\WINDOWS\.. 经 Clean
	// 落在盘根,① 驱动器根先于 ② 命中,Kind=DenyDriveRoot(与收口前旧 fs.go
	// 的 len<=3 规则同款)。
	Kind DenyKind
	// Norm Clean+Abs 归一化路径(归一化失败时为空串)。
	Norm string
	// Physical 判定主锚点:物理最终路径;打不开句柄时 = Norm(原始形态比对)。
	Physical string
	// PhysicalResolved 物理解析是否成功(false = 主锚点退回归一化原路径)。
	PhysicalResolved bool
	// Err 仅 DenyParseFail 时携带底层归一化错误,供调用方拼人话文案。
	Err error
}

// GuardDeny 是删除/移动护栏与协同工作区护栏共用的清单式七项判定,命中即拒:
//
//	① 驱动器根/文件系统根(含裸盘符 X:、设备形态 \\?\X:\ 变体);
//	② %SystemRoot%(默认 C:\Windows)根本身 + 顶层直接子项(仅一层);
//	③④⑤ %ProgramFiles% 与 %ProgramFiles(x86)% / %USERPROFILE%(或用户主目录)
//	   / os.TempDir() 根本身;
//	⑥ daemon 数据目录 —— dataDirs 参数(调用方自身的数据目录,可空)+ hostenv
//	   标准/旧版位置,唯一"整棵子树都拒"的条目;
//	⑦ 进程当前工作目录与可执行文件所在目录(仅目录本身)。
//
// 【锚点纪律】比对一律基于「物理最终路径」:候选与清单各根同款归一
// (physicalRoot),junction/符号链接/8.3/大小写在源头消解(缺口 G1 的落点)。
// 打不开句柄时主锚点退回归一化原路径,并叠加字符串层附加锚点:
//   - 设备前缀形态(\\?\C:\ 等)剥 \\?\ / \\?\UNC\ 前缀后比对(缺口 G3 的落点,
//     也兜住"异常环境下打不开盘根句柄"的漏检);
//   - 管理共享 UNC(\\<主机>\X$\...)把 X$ 别名映射回 X:\ 后比对(缺口 G4 的
//     落点:枚举管理共享需要管理员令牌,普通权限下物理解析必然失败,字符串层
//     归一是唯一途径;对远端主机的 X$ 同样生效 —— 删/移/占远端系统目录同样
//     不可逆,宁严勿松;普通命名共享 \\host\share 不受影响)。
//   - 裸盘符形态(X:,恰好两个字符)在 Clean/Abs 之前识别后按 ① 拒:
//     它是"盘符相对路径"引用(指向该盘当前目录),Abs 会把它归一成 <cwd>\X:
//     的无意义形态而漏检(缺口 G2 的落点;仅 Windows 生效,Unix 上 "C:" 是
//     合法相对文件名)。
func GuardDeny(path string, dataDirs []string) DenyResult {
	norm, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return DenyResult{Denied: true, Kind: DenyParseFail, Err: err}
	}
	if isBareDriveRoot(path) {
		return DenyResult{Denied: true, Kind: DenyDriveRoot, Norm: norm, Physical: norm}
	}
	physical, resolved := ResolvePhysical(norm)
	if !resolved {
		physical = norm
	}
	anchors := append([]string{physical}, extraAnchors(norm)...)
	// 命中即返回(首个命中:锚点序 × 清单①→⑦序;Kind 不做命中计数,口径见
	// DenyResult.Kind 注释)。
	for _, anchor := range anchors {
		if kind, hit := denyAnchor(anchor, dataDirs); hit {
			return DenyResult{
				Denied: true, Kind: kind,
				Norm: norm, Physical: physical, PhysicalResolved: resolved,
			}
		}
	}
	return DenyResult{Norm: norm, Physical: physical, PhysicalResolved: resolved}
}

// denyAnchor 对单个比对锚点跑清单①~⑦(顺序与历史实现一致,保证多命中时
// Kind 报告稳定)。返回 (命中条目, 是否命中)。
func denyAnchor(anchor string, dataDirs []string) (DenyKind, bool) {
	key := strings.ToLower(anchor)

	// ① 驱动器根(Windows)/文件系统根(Unix)
	if vol := filepath.VolumeName(anchor); vol != "" {
		if strings.EqualFold(anchor, vol+string(os.PathSeparator)) {
			return DenyDriveRoot, true
		}
	} else if anchor == string(os.PathSeparator) {
		return DenyDriveRoot, true
	}

	// ② %SystemRoot% 本身 + 直接子项(仅顶层一层;父子两侧都是物理形态)
	sysRoot := strings.TrimSpace(os.Getenv("SystemRoot"))
	if sysRoot == "" {
		sysRoot = `C:\Windows`
	}
	if sysKey := strings.ToLower(physicalRoot(sysRoot)); sysKey != "" {
		if key == sysKey {
			return DenySystemRoot, true
		}
		if strings.ToLower(filepath.Dir(anchor)) == sysKey {
			return DenySystemRootChild, true
		}
	}

	// ③④⑤ 程序目录 / 用户主目录 / 系统临时目录 —— 仅根本身
	homeDir := ""
	if h, herr := os.UserHomeDir(); herr == nil {
		homeDir = h
	}
	for _, root := range []string{
		os.Getenv("ProgramFiles"),
		os.Getenv("ProgramFiles(x86)"),
		os.Getenv("USERPROFILE"),
		homeDir,
		os.TempDir(),
	} {
		if root = strings.TrimSpace(root); root == "" {
			continue
		}
		if key == strings.ToLower(physicalRoot(root)) {
			return DenyHomeLikeRoot, true
		}
	}

	// ⑥ daemon 数据目录 —— 整棵子树都拒(内部账本无合法目标)。
	// 小写后再 Rel,与调用方既有的 rel 判定同款式样。
	for _, root := range dataRoots(dataDirs) {
		rel, rerr := filepath.Rel(strings.ToLower(physicalRoot(root)), key)
		if rerr != nil {
			continue // 跨盘等无法比较的情形:不命中
		}
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)) {
			return DenyDataDir, true
		}
	}

	// ⑦ 当前工作目录 / 可执行文件所在目录 —— 仅目录本身
	if wd, werr := os.Getwd(); werr == nil && wd != "" && key == strings.ToLower(physicalRoot(wd)) {
		return DenyProcessCwd, true
	}
	if exe, eerr := os.Executable(); eerr == nil {
		if key == strings.ToLower(physicalRoot(filepath.Dir(exe))) {
			return DenyExeDir, true
		}
	}
	return DenyNone, false
}

// dataRoots ⑥ 的清单根:调用方自身数据目录(空串过滤)+ hostenv 标准/旧版
// 位置(hostenv 归本包统一追加,调用方不必重复拼装)。
func dataRoots(extra []string) []string {
	roots := make([]string, 0, len(extra)+2)
	for _, d := range extra {
		if d = strings.TrimSpace(d); d != "" {
			roots = append(roots, d)
		}
	}
	return append(roots, hostenv.UserDataDir(), hostenv.LegacyDataDir())
}

// extraAnchors 候选路径的字符串层附加锚点(见 GuardDeny 锚点纪律):只增锚点
// 不改写主锚点,任一锚点命中即拒 —— fail-closed,不存在"多归一反而放行"的方向。
func extraAnchors(norm string) []string {
	var out []string
	if stripped := stripDevicePathPrefix(norm); stripped != norm {
		out = append(out, stripped)
	}
	if alias, ok := adminShareAlias(norm); ok {
		out = append(out, alias)
	}
	return out
}

// isBareDriveRoot 报告 path 是否是裸盘符形态(X:,恰好两个字符、无任何分隔符)。
// 仅 Windows 有意义(X: 在 Unix 是合法的相对文件名,不拦)。
func isBareDriveRoot(path string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	return len(path) == 2 && path[1] == ':' && isDriveLetter(path[0])
}

// adminShareAlias 把管理共享 UNC 形态(\\<主机>\X$\...)映射回本地盘符路径:
// \\localhost\C$\Windows → C:\Windows。X$ 是盘的远端视图,枚举需要管理员令牌,
// 普通权限下 ResolvePhysical 打不开句柄,只能在字符串层归一(缺口 G4 落点)。
// 手工解析而非 filepath.VolumeName:设备形态(\\?\UNC\...)的 VolumeName 切分
// 随 Go 版本有差异,这里显式按 \\主机\X$ 的字节形态判,行为不随版本漂移;
// share 名必须恰好是"单盘符+$",\\host\C$xyz 这类同名普通共享不误映射。
func adminShareAlias(norm string) (string, bool) {
	if runtime.GOOS != "windows" || !strings.HasPrefix(norm, `\\`) {
		return "", false
	}
	body := strings.TrimPrefix(norm, `\\`)
	slash := strings.Index(body, `\`)
	if slash <= 0 {
		return "", false // \\x(残缺)/ \\(裸)形态:不处理
	}
	rem := body[slash+1:]
	if len(rem) < 2 || rem[1] != '$' || !isDriveLetter(rem[0]) {
		return "", false // share 不是 X$ 形态:普通命名共享,不映射
	}
	tail := rem[2:]
	if tail != "" && tail[0] != '\\' {
		return "", false // \\host\C$xyz:以 X$ 开头的普通共享名,不映射
	}
	alias := string(rem[0]) + `:`
	if tail == "" {
		alias += `\`
	} else {
		alias += tail
	}
	return filepath.Clean(alias), true
}

func isDriveLetter(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}
