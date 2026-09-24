package hostenv

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const migrationMarkerName = "migration.json"

// MigrationResult is the single result consumed by daemon startup.
type MigrationResult struct {
	SelectedDir string
	Migrated    bool
	UsedLegacy  bool
	MarkerPath  string
}

type manifestEntry struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type fileManifest struct {
	Entries []manifestEntry
	Bytes   int64
	Hash    string
}

var migrationCopyTree = copyTree

// ResolveDataDir resolves the daemon data root before any store or provider is
// opened. An explicit path is authoritative and never triggers implicit
// migration. The default path migrates the legacy tree exactly once.
func ResolveDataDir(explicit string) (MigrationResult, error) {
	if strings.TrimSpace(explicit) != "" {
		dir, err := filepath.Abs(explicit)
		if err != nil {
			return MigrationResult{}, fmt.Errorf("解析显式数据目录失败: %w", err)
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return MigrationResult{}, fmt.Errorf("创建显式数据目录失败: %w", err)
		}
		return MigrationResult{SelectedDir: dir, MarkerPath: filepath.Join(dir, migrationMarkerName)}, nil
	}

	newDir := UserDataDir()
	oldDir := LegacyDataDir()
	if st, err := os.Stat(newDir); err == nil {
		if !st.IsDir() {
			return MigrationResult{}, fmt.Errorf("新数据目录不是目录: %s", newDir)
		}
		return MigrationResult{SelectedDir: newDir, MarkerPath: filepath.Join(newDir, migrationMarkerName)}, nil
	} else if !os.IsNotExist(err) {
		return MigrationResult{}, fmt.Errorf("检查新数据目录失败: %w", err)
	}
	if st, err := os.Stat(oldDir); err != nil {
		if !os.IsNotExist(err) {
			return MigrationResult{}, fmt.Errorf("检查旧数据目录失败: %w", err)
		}
		if err := os.MkdirAll(newDir, 0o700); err != nil {
			return MigrationResult{}, fmt.Errorf("创建新数据目录失败: %w", err)
		}
		return MigrationResult{SelectedDir: newDir, MarkerPath: filepath.Join(newDir, migrationMarkerName)}, nil
	} else if !st.IsDir() {
		return MigrationResult{}, fmt.Errorf("旧数据路径不是目录: %s", oldDir)
	}

	if err := migrateTree(oldDir, newDir); err != nil {
		return MigrationResult{}, err
	}
	return MigrationResult{
		SelectedDir: newDir,
		Migrated:    true,
		MarkerPath:  filepath.Join(newDir, migrationMarkerName),
	}, nil
}

type migrationMarker struct {
	SchemaVersion  int    `json:"schema_version"`
	From           string `json:"from"`
	To             string `json:"to"`
	CompletedAtMs  int64  `json:"completed_at_unix_ms"`
	FileCount      int    `json:"file_count"`
	Bytes          int64  `json:"bytes"`
	ManifestSHA256 string `json:"manifest_sha256"`
}

func migrateTree(from, to string) error {
	parent := filepath.Dir(to)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return fmt.Errorf("创建迁移父目录失败: %w", err)
	}
	tmp, err := os.MkdirTemp(parent, ".pocketagent-data-migrate-*")
	if err != nil {
		return fmt.Errorf("创建迁移临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmp)

	sourceManifest, err := buildManifest(from)
	if err != nil {
		return fmt.Errorf("读取旧数据清单失败: %w", err)
	}
	targetManifest, err := migrationCopyTree(from, tmp)
	if err != nil {
		return fmt.Errorf("复制旧数据失败，旧目录保留: %w", err)
	}
	if sourceManifest.Hash != targetManifest.Hash ||
		sourceManifest.Bytes != targetManifest.Bytes ||
		len(sourceManifest.Entries) != len(targetManifest.Entries) {
		return fmt.Errorf("迁移校验失败，旧目录保留: source files=%d bytes=%d hash=%s target files=%d bytes=%d hash=%s",
			len(sourceManifest.Entries), sourceManifest.Bytes, sourceManifest.Hash,
			len(targetManifest.Entries), targetManifest.Bytes, targetManifest.Hash)
	}

	marker := migrationMarker{
		SchemaVersion:  1,
		From:           from,
		To:             to,
		CompletedAtMs:  time.Now().UnixMilli(),
		FileCount:      len(targetManifest.Entries),
		Bytes:          targetManifest.Bytes,
		ManifestSHA256: targetManifest.Hash,
	}
	if err := writeJSONAtomic(filepath.Join(tmp, migrationMarkerName), marker); err != nil {
		return fmt.Errorf("写迁移 marker 失败，旧目录保留: %w", err)
	}
	if err := syncDir(tmp); err != nil {
		return fmt.Errorf("同步迁移临时目录失败，旧目录保留: %w", err)
	}
	if _, err := os.Stat(to); err == nil {
		return fmt.Errorf("迁移目标在切换前已出现，拒绝覆盖: %s", to)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查迁移目标失败: %w", err)
	}
	if err := os.Rename(tmp, to); err != nil {
		return fmt.Errorf("原子切换迁移目录失败，旧目录保留: %w", err)
	}
	if err := syncDir(parent); err != nil {
		return fmt.Errorf("同步迁移父目录失败: %w", err)
	}
	return nil
}

func copyTree(from, to string) (fileManifest, error) {
	if err := os.MkdirAll(to, 0o700); err != nil {
		return fileManifest{}, err
	}
	err := filepath.WalkDir(from, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == from {
			return nil
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		if isTransientMigrationPath(entry.Name()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		dst := filepath.Join(to, rel)
		if entry.IsDir() {
			return os.MkdirAll(dst, 0o700)
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		if err := copyFile(path, dst, entry); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fileManifest{}, err
	}
	return buildManifest(to)
}

func copyFile(from, to string, entry os.DirEntry) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	if err := os.MkdirAll(filepath.Dir(to), 0o700); err != nil {
		return err
	}
	dst, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return err
	}
	if err := dst.Sync(); err != nil {
		_ = dst.Close()
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	if info, err := entry.Info(); err == nil {
		_ = os.Chmod(to, info.Mode().Perm())
	}
	return nil
}

func buildManifest(root string) (fileManifest, error) {
	var entries []manifestEntry
	var total int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if isTransientMigrationPath(entry.Name()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		sum, bytes, err := hashFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		entries = append(entries, manifestEntry{
			Path: filepath.ToSlash(rel), Bytes: bytes, SHA256: sum,
		})
		total += bytes
		return nil
	})
	if err != nil {
		return fileManifest{}, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	raw, err := json.Marshal(entries)
	if err != nil {
		return fileManifest{}, err
	}
	hash := sha256.Sum256(raw)
	return fileManifest{
		Entries: entries,
		Bytes:   total,
		Hash:    hex.EncodeToString(hash[:]),
	}, nil
}

func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func isTransientMigrationPath(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".tmp") ||
		strings.HasSuffix(lower, ".partial") ||
		strings.HasSuffix(lower, ".lock")
}

func writeJSONAtomic(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	if err := syncPath(tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return syncDir(filepath.Dir(path))
}

func syncPath(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func syncDir(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
