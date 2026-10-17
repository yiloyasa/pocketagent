package agent

// DeletePiSession removes only the exact native session under Pi's sessions
// directory. A stale/mismatching reference must not delete an unrelated file.
func DeletePiSession(file, sessionID string) error {
	return deletePiSessionFrom(PiSessionsDir(), file, sessionID)
}

func deletePiSessionFrom(root, file, sessionID string) error {
	if file == "" {
		return nil // A new thread may never have created a native session.
	}
	// 四道闸(解析 → 包含性 → 扩展名 → 文件头 id)走共享闸门 session_delete_guard.go:
	// pi 与 codex 删的是同一类东西,守卫只能有一份真值;pi 侧只负责回答"pi 的文件头
	// 长什么样"(piSessionHeadID)这一件事。
	return deleteNativeSessionFile(root, file, sessionID, "Pi", piSessionHeadID)
}

// piSessionHeadID 读 pi 会话文件头里的会话 id(type=="session" 行)。复用
// scanPiSessionHead,不另写一套文件头解析 —— pi 的文件头格式若变了,这里自动跟着变。
func piSessionHeadID(path string) string {
	var info PiSessionInfo
	scanPiSessionHead(path, &info)
	return info.SessionID
}
