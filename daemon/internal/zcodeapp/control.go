package zcodeapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// RPCError preserves native error codes across CallCtx's method annotation.
type RPCError struct {
	Code    int
	Message string
	Data    json.RawMessage
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("ZCode 返回错误 %d: %s", e.Code, e.Message)
}

func (c *Client) CloseSession(ctx context.Context, id string) error {
	raw, err := c.CallCtx(ctx, "session/close", map[string]string{"sessionId": id})
	var rpc *RPCError
	if errors.As(err, &rpc) && rpc.Code == -32004 {
		return nil
	}
	if err != nil {
		return err
	}
	var result struct {
		Closed *bool `json:"closed"`
	}
	if json.Unmarshal(raw, &result) != nil || result.Closed == nil || !*result.Closed {
		return errors.New("ZCode session/close 未确认删除")
	}
	return nil
}

// ModeSchema queries validation only: required sessionId is absent, so this
// request cannot change a session. Unknown schema is reported, never guessed.
func (c *Client) ModeSchema(ctx context.Context) ([]string, error) {
	_, err := c.CallCtx(ctx, "session/setMode", map[string]any{})
	return parseModeSchema(err)
}

var modeEnumValue = regexp.MustCompile(`"([^"]+)"`)

func parseModeSchema(err error) ([]string, error) {
	var rpc *RPCError
	if !errors.As(err, &rpc) || rpc.Code != -32602 {
		if err == nil {
			err = errors.New("ZCode 未返回 mode schema")
		}
		return nil, err
	}
	_, mode, ok := strings.Cut(rpc.Message, "mode: Invalid option: expected one of ")
	if !ok {
		return nil, errors.New("ZCode mode schema 无法识别")
	}
	mode, _, _ = strings.Cut(mode, ";")
	var values []string
	for _, match := range modeEnumValue.FindAllStringSubmatch(mode, -1) {
		values = append(values, match[1])
	}
	if len(values) == 0 {
		return nil, errors.New("ZCode mode schema 没有可选项")
	}
	return values, nil
}
