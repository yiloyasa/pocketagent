package zcodeapp

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBuiltinConfigMatchesInstalledVersionAndEndpointNotMtime(t *testing.T) {
	root := t.TempDir()
	resources := filepath.Join(root, "resources")
	cjs := filepath.Join(resources, "glm", "zcode.cjs")
	v2 := filepath.Join(root, "v2")
	write := func(p string, b []byte) {
		t.Helper()
		if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(p, b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	pkg := []byte(`{"version":"9.2.1"}`)
	header, _ := json.Marshal(map[string]any{"files": map[string]any{"package.json": map[string]any{"offset": "0", "size": len(pkg)}}})
	lead := make([]byte, 16)
	binary.LittleEndian.PutUint32(lead[4:8], uint32(8+len(header)))
	binary.LittleEndian.PutUint32(lead[12:16], uint32(len(header)))
	write(filepath.Join(resources, "app.asar"), append(append(lead, header...), pkg...))
	t.Setenv("ZCODE_BASE_URL", "https://endpoint.example")
	bundled := filepath.Join(resources, "config", "provider", "zcode-builtin.json")
	release := func(revision int) []byte {
		return []byte(fmt.Sprintf(`{"schemaVersion":1,"revision":%d,"config":{}}`, revision))
	}
	write(bundled, release(7))
	hash := sha256.Sum256([]byte("https://endpoint.example"))
	platform := runtime.GOOS
	if platform == "darwin" {
		platform = "macos"
	}
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	} else if arch == "arm64" {
		arch = "aarch64"
	}
	cacheRoot := filepath.Join(v2, "runtime", "provider", platform+"-"+arch)
	match := filepath.Join(cacheRoot, "9.2.1", fmt.Sprintf("endpoint-%x", hash[:16]), "zcode-builtin.json")
	write(match, release(8))
	write(filepath.Join(cacheRoot, "99.0.0", "endpoint-other", "zcode-builtin.json"), release(99))
	write(filepath.Join(cacheRoot, "9.2.1", "endpoint-wrong", "zcode-builtin.json"), release(99))
	if got := builtinProviderConfigForCJS(cjs, v2); got != match {
		t.Fatalf("selected unrelated source: %s", got)
	}
	write(match, release(6))
	if got := builtinProviderConfigForCJS(cjs, v2); got != bundled {
		t.Fatalf("downgraded below bundled revision: %s", got)
	}
}
