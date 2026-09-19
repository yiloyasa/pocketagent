package zcodeapp

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Match the desktop's FKe/resolveZCodeBuiltinCachePaths: platform + installed
// app version + SHA256(endpoint origin). A newer mtime from another endpoint or
// version is not authority. Never edit or refresh these user-owned sources.
func scopedBuiltinProviderConfig(cjs, v2 string) string {
	resources := filepath.Clean(filepath.Join(filepath.Dir(cjs), ".."))
	bundled := filepath.Join(resources, "config", "provider", "zcode-builtin.json")
	version := asarPackageVersion(filepath.Join(resources, "app.asar"))
	if version != "" {
		origin := zcodeEndpointOrigin(v2)
		u, err := url.Parse(origin)
		if err == nil && u.Host != "" && u.User == nil && (u.Scheme == "https" || u.Scheme == "http") {
			host := strings.ToLower(u.Host)
			if (u.Scheme == "https" && u.Port() == "443") || (u.Scheme == "http" && u.Port() == "80") {
				host = u.Hostname()
			}
			hash := sha256.Sum256([]byte(strings.ToLower(u.Scheme) + "://" + host))
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
			active := filepath.Join(v2, "runtime", "provider", platform+"-"+arch, version, fmt.Sprintf("endpoint-%x", hash[:16]), "zcode-builtin.json")
			br, bok := builtinReleaseRevision(bundled)
			ar, aok := builtinReleaseRevision(active)
			if aok && (!bok || ar > br) {
				return active
			}
			// Same revision: prefer bundled, as the native source does when
			// equal revisions have different contents.
		}
	}
	if fileExists(bundled) {
		return bundled
	}
	return ""
}

func builtinReleaseRevision(path string) (int64, bool) {
	var w struct {
		SchemaVersion int             `json:"schemaVersion"`
		Revision      *int64          `json:"revision"`
		Config        json.RawMessage `json:"config"`
	}
	raw, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(raw, &w) != nil || w.SchemaVersion != 1 || w.Revision == nil || *w.Revision < 0 || len(w.Config) == 0 {
		return 0, false
	}
	return *w.Revision, true
}

func zcodeEndpointOrigin(v2 string) string {
	for _, key := range []string{"ZCODE_BASE_URL", "ZCODE_ENDPOINT_ORIGIN"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	env := "PRODUCTION"
	fallback := "https://zcode.z.ai"
	if strings.EqualFold(os.Getenv("ZCODE_ENV"), "test") {
		env = "TEST"
		fallback = "https://zcode.chatglm.site"
	}
	if v := strings.TrimSpace(os.Getenv("ZCODE_" + env + "_BASE_URL")); v != "" {
		return v
	}
	var settings struct {
		Origin string `json:"zcodeEndpointOrigin"`
	}
	if readAccountJSON(filepath.Join(v2, "setting.json"), &settings) == nil && strings.TrimSpace(settings.Origin) != "" {
		return settings.Origin
	}
	return fallback
}

// Read only package.json from Electron's ASAR container. No Electron launch,
// extraction, evaluation, or dependency on a separately installed ASAR tool.
func asarPackageVersion(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	header := make([]byte, 16)
	if _, err = io.ReadFull(f, header); err != nil {
		return ""
	}
	size := int64(binary.LittleEndian.Uint32(header[12:16]))
	base := int64(binary.LittleEndian.Uint32(header[4:8])) + 8
	if size <= 0 || size > 4*1024*1024 || base < size+16 {
		return ""
	}
	raw := make([]byte, size)
	if _, err = io.ReadFull(f, raw); err != nil {
		return ""
	}
	var index struct {
		Files map[string]struct {
			Offset string `json:"offset"`
			Size   int64  `json:"size"`
		} `json:"files"`
	}
	if json.Unmarshal(raw, &index) != nil {
		return ""
	}
	entry := index.Files["package.json"]
	offset, err := strconv.ParseInt(entry.Offset, 10, 64)
	if err != nil || offset < 0 || entry.Size <= 0 || entry.Size > 1024*1024 {
		return ""
	}
	raw = make([]byte, entry.Size)
	if _, err = f.ReadAt(raw, base+offset); err != nil {
		return ""
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(raw, &pkg) != nil || pkg.Version == "" || pkg.Version == "." || pkg.Version == ".." || strings.ContainsAny(pkg.Version, `\/`) {
		return ""
	}
	return pkg.Version
}
