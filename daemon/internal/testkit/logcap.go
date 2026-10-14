package testkit

import (
	"bytes"
	"log"
	"os"
)

// CaptureLog 劫持标准 log 输出,返回读取函数与恢复函数。
//
// 正文原样收编自 eventstore/eventstore_degrade_test.go 与 term/manager_idle_test.go
// 的同名 captureLog:两个包抓的都是"降级/清扫必须留下告警日志"这类断言,口径必须
// 一致(恢复一律回 os.Stderr,与标准 log 默认输出相同)。
func CaptureLog() (read func() string, restore func()) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	return buf.String, func() { log.SetOutput(os.Stderr) }
}
