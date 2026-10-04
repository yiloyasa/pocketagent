// bridgehealth verifies the bridge using the existing local certificate pin.
// It never changes credentials, VPN settings, native models or conversation input.
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/hostenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// certPath 定位 daemon 的自签证书:优先迁移后的数据目录
// (<LOCALAPPDATA>\PocketAgent\data),回退历史位置(家目录 .pocketagentd)。
// 只读两级探测,与 cmd/wbempty 的 storePath 同款:现存的优先,都不存在时
// 回退历史位置,让读失败的报错仍指向一个说得通的位置。不调
// hostenv.ResolveDataDir —— 那会触发一次性迁移副作用,诊断工具不该替
// daemon 做迁移。
func certPath() string {
	for _, dir := range []string{hostenv.UserDataDir(), hostenv.LegacyDataDir()} {
		p := filepath.Join(dir, "certs", "server-cert.pem")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join(hostenv.LegacyDataDir(), "certs", "server-cert.pem")
}

func main() {
	addr := flag.String("addr", "127.0.0.1:7246", "daemon endpoint")
	thread := flag.String("thread", "", "optional thread to reconcile with its native terminal record")
	agentID := flag.String("agent", "", "optional native model catalog and settings to read")
	flag.Parse()
	raw, err := os.ReadFile(certPath())
	if err != nil {
		log.Fatal(err)
	}
	cert, _ := pem.Decode(raw)
	if cert == nil {
		log.Fatal("invalid local certificate")
	}
	creds := credentials.NewTLS(&tls.Config{
		InsecureSkipVerify: true, // Exact DER pin is checked below, including over LAN/Tailscale.
		MinVersion:         tls.VersionTLS12,
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 || !bytes.Equal(state.PeerCertificates[0].Raw, cert.Bytes) {
				return fmt.Errorf("daemon certificate pin mismatch")
			}
			return nil
		},
	})
	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	host, err := pb.NewHostServiceClient(conn).GetHostInfo(ctx, &pb.GetHostInfoRequest{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("HOST version=%s tls_pin=verified lan=%s tailscale=%s auth_required=%t\n",
		host.DaemonVersion, host.LanIpv4, host.TailscaleIpv4, host.AuthRequired)
	fmt.Printf("CAPABILITIES %v\n", host.Capabilities)
	if *agentID != "" {
		client := pb.NewModelServiceClient(conn)
		catalog, err := client.ListModels(ctx, &pb.ListModelsRequest{AgentId: *agentID, Refresh: true, ThreadId: *thread})
		if err != nil {
			log.Fatal(err)
		}
		for _, model := range catalog.Models {
			fmt.Printf("MODEL id=%s label=%s levels=%v context=%d\n", model.Id, model.DisplayName, model.AvailableEfforts, model.ContextWindow)
		}
		if catalog.CurrentModelId != "" {
			fmt.Printf("CURRENT model=%s effort=%s\n", catalog.CurrentModelId, catalog.GetCurrentEffort())
		}
		settings, err := client.GetCodexSettings(ctx, &pb.GetCodexSettingsRequest{AgentId: *agentID, ThreadId: *thread})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("NATIVE_MODES selected=%s choices=%v\n", settings.NativeMode, settings.NativeModes)
	}
	if *thread != "" {
		t, err := pb.NewThreadServiceClient(conn).GetThread(ctx, &pb.GetThreadRequest{ThreadId: *thread})
		if err != nil {
			log.Fatal(err)
		}
		// Deliberately do not print user conversation text or upstream credentials.
		fmt.Printf("THREAD id=%s agent=%s status=%s messages=%d native_error_present=%t\n",
			t.Id, t.AgentId, t.Status, t.MessageCount, t.LastTurnError != "")
	}
}
