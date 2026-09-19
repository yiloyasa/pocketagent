// mdnsprobe:mDNS 自测工具(P0-3)。
//
// 浏览局域网内的 _pocketagent._tcp 服务 5 秒,打印发现的全部实例
// (实例名 / IP / 端口 / TXT)。用途:验证 daemon 的 mDNS 广播是否可达,
// 也用于排查"手机发现不了电脑"时到底是广播侧还是解析侧的问题。
//
// 用法:
//
//	mdnsprobe [-type _pocketagent._tcp] [-timeout 5s]
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/pocketagent/daemon/internal/mdns"
)

func main() {
	svc := flag.String("type", mdns.Service, "要浏览的服务类型")
	timeout := flag.Duration("timeout", 5*time.Second, "浏览时长")
	flag.Parse()

	fmt.Printf("浏览 %s.%s.(%ds)...\n", *svc, mdns.Domain, int(timeout.Seconds()))
	start := time.Now()
	entries, err := mdns.Browse(context.Background(), *timeout, nil)
	if err != nil {
		log.Fatalf("浏览失败: %v", err)
	}
	fmt.Printf("耗时 %s, 发现 %d 个实例:\n", time.Since(start).Round(time.Millisecond), len(entries))
	for _, e := range entries {
		fmt.Printf("  - 实例名: %s\n", e.Instance)
		fmt.Printf("    主机:   %s\n", e.Host)
		fmt.Printf("    地址:   %s (IPs: %v)\n", e.AddrString(), e.IPs)
		fmt.Printf("    端口:   %d\n", e.Port)
		fmt.Printf("    TXT:    v=%s id=%s name=%s tls=%s\n",
			e.Text[mdns.TXTKeyVersion], e.HostID(), e.DisplayName(), e.Text[mdns.TXTKeyTLS])
	}
}
