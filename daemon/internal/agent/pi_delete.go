package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DeletePiSession removes only the exact native session under Pi's sessions
// directory. A stale/mismatching reference must not delete an unrelated file.
func DeletePiSession(file, sessionID string) error {
	return deletePiSessionFrom(PiSessionsDir(), file, sessionID)
}

func deletePiSessionFrom(root, file, sessionID string) error {
	if file == "" {
		return nil // A new thread may never have created a native session.
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(file)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(resolvedRoot, resolved)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("Pi 会话路径不在 sessions 目录中")
	}
	if !strings.EqualFold(filepath.Ext(resolved), ".jsonl") {
		return fmt.Errorf("不是 Pi 会话文件")
	}
	var info PiSessionInfo
	scanPiSessionHead(resolved, &info)
	if sessionID == "" || info.SessionID != sessionID {
		return fmt.Errorf("Pi 会话 ID 与文件头不一致，未删除")
	}
	return os.Remove(resolved)
}
