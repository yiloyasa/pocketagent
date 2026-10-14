// Package version holds the version shared by daemon diagnostics and the
// Codex app-server client identity.
//
// Version is the single source of truth. It is a var (not a const) so the
// release build can stamp it into the binary at link time:
//
//	go build -ldflags "-X github.com/pocketagent/daemon/internal/version.Version=x.y.z" ./cmd/pocketagentd
//
// The Go linker's -X flag can only rewrite string vars initialized to a
// constant; a const would silently keep the default value in the binary.
package version

// Version is the daemon version. Default value stays in sync with the
// repository; dist/windows/build.ps1 stamps the same value at build time
// (and also drops it into version.txt inside the zip for reconciliation).
var Version = "0.4.0"

// App is the historical name kept for existing callers (internal/server/host.go,
// internal/agent/codex.go). It derives from Version, so a -ldflags stamp on
// Version is visible through App as well.
var App = Version

// MinAppVersion is the lowest App (APK) version this daemon accepts working
// with, reported to the phone via HostInfo.min_app_version (host.proto field 13).
// Empty string would mean "no floor"; we always set an explicit value so the
// phone-side connection diagnostics can flag an outdated App ("App 版本过旧").
// The comparison is advisory only: the App shows a hint, it never hard-blocks.
//
// 【升级纪律】本仓库只维护最新版，不兼容旧格式：每次发版 MinAppVersion
// 与 Version 同步抬到当前版本号 —— 旧 APK / 旧 daemon 不再保证可连，
// App 侧仅作提示不硬拦。比较函数见 ConnectionDiagnostics.isAppVersionTooOld。
const MinAppVersion = "0.4.0"
