package main

import (
	"fmt"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
)

// zcsessprobe:真机同链路实证 ScanZcodeSessions(daemon 进程内,同 Start(Temp) 链)。
func main() {
	start := time.Now()
	sessions, err := agent.ScanZcodeSessions()
	elapsed := time.Since(start)
	if err != nil {
		fmt.Printf("FAIL %v (%s)\n", err, elapsed)
		return
	}
	fmt.Printf("OK %d sessions (%s)\n", len(sessions), elapsed)
	for i, s := range sessions {
		if i >= 5 {
			break
		}
		fmt.Printf("  - %s | %s\n", s.Title, s.UpdatedAt.Format("15:04:05"))
	}
}
