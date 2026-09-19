package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// Optional adapter contract: a transport failure is not evidence of completion.
// Only a native terminal record newer than the local running turn can settle it.
type NativeTurnResult struct {
	TurnID string
	At     int64
	Text   string
	Error  *pb.Error
}
type NativeTurnReader interface {
	ReadNativeTurn(context.Context, *store.Thread) (*NativeTurnResult, error)
}

func (*CodexProvider) ReadNativeTurn(ctx context.Context, t *store.Thread) (*NativeTurnResult, error) {
	if t.CodexThreadID == "" {
		return nil, nil
	}
	path, err := FindCodexSession(t.CodexThreadID)
	if err != nil {
		return nil, err
	}
	return readCodexNativeTurn(ctx, path, t.UpdatedAt)
}

func readCodexNativeTurn(ctx context.Context, path string, after int64) (*NativeTurnResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var last *NativeTurnResult
	var text string
	var turnID string
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var row struct {
			Type      string `json:"type"`
			Timestamp string `json:"timestamp"`
			Payload   struct {
				Type             string          `json:"type"`
				TurnID           string          `json:"turn_id"`
				Message          string          `json:"message"`
				LastAgentMessage *string         `json:"last_agent_message"`
				Error            json.RawMessage `json:"error"`
			} `json:"payload"`
		}
		if json.Unmarshal(scanner.Bytes(), &row) != nil || row.Type != "event_msg" {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, row.Timestamp)
		if err != nil || at.UnixMilli() < after {
			continue
		}
		switch row.Payload.Type {
		case "task_started":
			last, text, turnID = nil, "", row.Payload.TurnID
		case "agent_message":
			text = row.Payload.Message
		case "task_complete", "turn_aborted":
			if row.Payload.LastAgentMessage != nil {
				text = *row.Payload.LastAgentMessage
			}
			last = &NativeTurnResult{TurnID: firstNonEmpty(row.Payload.TurnID, turnID), At: at.UnixMilli(), Text: text}
			if raw := row.Payload.Error; len(raw) > 0 && string(raw) != "null" {
				var native struct {
					Message string `json:"message"`
					Code    string `json:"code"`
				}
				_ = json.Unmarshal(raw, &native)
				if native.Message == "" {
					_ = json.Unmarshal(raw, &native.Message)
				}
				if native.Message == "" {
					native.Message = "原生 Agent 已结束任务并报告错误"
				}
				last.Error = &pb.Error{Code: firstNonEmpty(native.Code, "NATIVE_ERROR"), Message: native.Message}
			} else if row.Payload.Type == "turn_aborted" {
				last.Error = &pb.Error{Code: "CANCELLED", Message: "任务已在原生 Agent 中停止"}
			}
			last.Text = strings.TrimSpace(last.Text)
		}
	}
	return last, scanner.Err()
}
