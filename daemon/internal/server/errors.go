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
