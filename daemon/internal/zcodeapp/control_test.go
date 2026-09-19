package zcodeapp

import (
	"fmt"
	"reflect"
	"testing"
)

func TestModeSchemaIsNativeAndOpenEnded(t *testing.T) {
	err := fmt.Errorf("session/setMode: %w", &RPCError{Code: -32602, Message: `Invalid params — sessionId: missing; mode: Invalid option: expected one of "plan"|"build"|"future-mode"`})
	modes, e := parseModeSchema(err)
	if e != nil || !reflect.DeepEqual(modes, []string{"plan", "build", "future-mode"}) {
		t.Fatalf("modes=%v err=%v", modes, e)
	}
	for _, err := range []error{nil, &RPCError{Code: -32004, Message: "not found"}, &RPCError{Code: -32602, Message: "changed schema"}} {
		if _, e := parseModeSchema(err); e == nil {
			t.Fatalf("guessed modes from %v", err)
		}
	}
}
