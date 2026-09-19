//go:build !windows
// +build !windows

// ctrl_other.go:非 Windows 平台的输入写入路径。
// Unix 伪终端上 0x03 由 termios ISIG 直接翻译成前台进程组的 SIGINT,
// 无需任何翻译,原样写入即可(保留对称接口便于跨平台调用)。
package term

// writeInput 直接原样写入(Unix 内核态 line discipline 处理 Ctrl+C)。
func (s *Session) writeInput(data []byte) error {
	_, err := s.pty.Write(data)
	return err
}
