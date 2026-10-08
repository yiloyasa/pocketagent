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
var Version = "0.3.0"

// App is the historical name kept for existing callers (internal/server/host.go,
// internal/agent/codex.go). It derives from Version, so a -ldflags stamp on
// Version is visible through App as well.
var App = Version
