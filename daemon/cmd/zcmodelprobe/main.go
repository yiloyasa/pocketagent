package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/pocketagent/daemon/internal/zcodeapp"
)

type modelView struct {
	Ref struct {
		ProviderID string `json:"providerId"`
		ModelID    string `json:"modelId"`
	} `json:"ref"`
	Label         string `json:"label"`
	ProviderLabel string `json:"providerLabel"`
	Reasoning     struct {
		DefaultLevel string            `json:"defaultLevel"`
		Levels       []json.RawMessage `json:"levels"`
	} `json:"reasoning"`
}

func main() {
	builtin := flag.String("builtin", "", "explicit ZCODE_BUILTIN_PROVIDER_CONFIG_FILE")
	personal := flag.String("personal", "", "explicit ZCODE_PERSONAL_PROVIDER_CONFIG_FILE")
	generate := flag.Bool("generate", false, "verify first native account model with a small text-only call")
	modes := flag.Bool("modes", false, "exercise native mode schema in an isolated temporary workspace")
	flag.Parse()
	if *builtin != "" {
		_ = os.Setenv("ZCODE_BUILTIN_PROVIDER_CONFIG_FILE", *builtin)
	}
	if *personal != "" {
		_ = os.Setenv("ZCODE_PERSONAL_PROVIDER_CONFIG_FILE", *personal)
	}
	node, cjs, err := zcodeapp.LocateZCode()
	if err != nil {
		panic(err)
	}
	fmt.Printf("node=%s\ncjs=%s\nbuiltin=%s\npersonal=%s\n", node, cjs, *builtin, *personal)
	workspace, err := os.MkdirTemp("", "pa-zcode-probe-")
	if err != nil {
		panic(err)
	}
	defer os.Remove(workspace)
	c, err := zcodeapp.Start(zcodeapp.StartOptions{Workspace: workspace})
	if err != nil {
		panic(err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	raw, err := c.CallCtx(ctx, "session/create", map[string]any{
		"workspace": map[string]string{
			"workspacePath": workspace,
			"workspaceKey":  workspace,
		},
		"mode": "build",
	})
	if err != nil {
		panic(err)
	}
	var snap struct {
		Session struct {
			SessionID string `json:"sessionId"`
		} `json:"session"`
		Settings struct {
			Model struct {
				Current struct {
					ProviderID string `json:"providerId"`
					ModelID    string `json:"modelId"`
				} `json:"current"`
				Available []modelView `json:"available"`
			} `json:"model"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		panic(err)
	}
	fmt.Printf("session=%s current=%s/%s\n", snap.Session.SessionID, snap.Settings.Model.Current.ProviderID, snap.Settings.Model.Current.ModelID)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		fmt.Printf("deferred_close_error=%v\n", c.CloseSession(ctx, snap.Session.SessionID))
	}()
	var safe map[string]json.RawMessage
	_ = json.Unmarshal(raw, &safe)
	var settings map[string]json.RawMessage
	_ = json.Unmarshal(safe["settings"], &settings)
	for _, key := range []string{"mode", "permission", "thoughtLevel"} {
		fmt.Printf("settings.%s=%s\n", key, settings[key])
	}
	var model map[string]json.RawMessage
	_ = json.Unmarshal(settings["model"], &model)
	fmt.Printf("current=%s\n", model["current"])
	for _, method := range []string{"runtime/capabilities", "session/setMode"} {
		v, e := c.CallCtx(ctx, method, map[string]any{})
		fmt.Printf("schema %s result=%s error=%v\n", method, v, e)
	}
	var modeValues []string
	if *modes {
		modeValues, err = c.ModeSchema(ctx)
		if err != nil {
			panic(err)
		}
	}
	for _, mode := range modeValues {
		v, e := c.CallCtx(ctx, "session/setMode", map[string]any{"sessionId": snap.Session.SessionID, "mode": mode})
		var w struct {
			Projection struct {
				Mode string `json:"mode"`
			} `json:"projection"`
			Settings struct {
				Mode json.RawMessage `json:"mode"`
			} `json:"settings"`
		}
		_ = json.Unmarshal(v, &w)
		fmt.Printf("setMode=%s projection=%s settings=%s error=%v\n", mode, w.Projection.Mode, w.Settings.Mode, e)
	}
	sort.Slice(snap.Settings.Model.Available, func(i, j int) bool {
		a, b := snap.Settings.Model.Available[i], snap.Settings.Model.Available[j]
		if a.Ref.ProviderID != b.Ref.ProviderID {
			return a.Ref.ProviderID < b.Ref.ProviderID
		}
		return a.Ref.ModelID < b.Ref.ModelID
	})
	fmt.Printf("available=%d\n", len(snap.Settings.Model.Available))
	for _, m := range snap.Settings.Model.Available {
		fmt.Printf("model provider=%s id=%s label=%s provider_label=%s default_level=%s\n",
			m.Ref.ProviderID, m.Ref.ModelID, m.Label, m.ProviderLabel, m.Reasoning.DefaultLevel)
	}
	if *generate {
		for _, m := range snap.Settings.Model.Available {
			if !strings.HasPrefix(m.Ref.ProviderID, "account:") {
				continue
			}
			selection := map[string]any{"providerId": m.Ref.ProviderID, "modelId": m.Ref.ModelID}
			if m.Reasoning.DefaultLevel != "" {
				selection["options"] = map[string]string{"reasoningLevel": m.Reasoning.DefaultLevel}
			}
			gctx, gcancel := context.WithTimeout(context.Background(), 90*time.Second)
			result, e := c.CallCtx(gctx, "workspace/generateText", map[string]any{
				"workspace": map[string]string{"workspacePath": workspace, "workspaceKey": workspace},
				"selection": selection, "prompt": "Reply with exactly PONG.", "querySource": "pocketagent-account-probe", "maxOutputTokens": 4096,
			})
			gcancel()
			var reply struct {
				Text string `json:"text"`
			}
			_ = json.Unmarshal(result, &reply)
			fmt.Printf("account_generate provider=%s model=%s pong=%t error=%v\n", m.Ref.ProviderID, m.Ref.ModelID, strings.Contains(reply.Text, "PONG"), e)
			break
		}
	}
	if snap.Session.SessionID != "" {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer closeCancel()
		if _, err := c.CallCtx(closeCtx, "session/close", map[string]string{"sessionId": snap.Session.SessionID}); err != nil {
			fmt.Printf("close_error=%v\n", err)
		} else {
			fmt.Println("closed=true")
		}
	}
}
