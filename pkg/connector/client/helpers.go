package client

import (
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const alreadyInvitedMessage = "was already invited to join workspace"

func IsAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	if status.Code(err) == codes.AlreadyExists {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), alreadyInvitedMessage)
}
