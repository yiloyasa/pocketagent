package server

import (
	"os"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func hostDisplayName() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "PocketAgent Host"
	}
	return h
}

func errPermissionDenied(msg string) error { return status.Error(codes.PermissionDenied, msg) }
func errNotFound(msg string) error         { return status.Error(codes.NotFound, msg) }
func errInvalidArg(msg string) error       { return status.Error(codes.InvalidArgument, msg) }
func errInternal(err error) error          { return status.Error(codes.Internal, err.Error()) }

// errUnavailable:能力整块不可用(没装 codex / 没登录)。
// 手机端据此显示"去电脑上配置",而不是当故障重试。
func errUnavailable(msg string) error { return status.Error(codes.Unavailable, msg) }

// errFailedPrecondition:请求本身合法,但当前状态不允许(会话还没在电脑上落地、
// 审查结论已过期)。这类失败重试没用,手机端应提示用户"换个做法"。
func errFailedPrecondition(msg string) error { return status.Error(codes.FailedPrecondition, msg) }
