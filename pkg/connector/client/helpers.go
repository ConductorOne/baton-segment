package client

import "strings"

const alreadyInvitedMessage = "was already invited to join workspace"

func IsAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), alreadyInvitedMessage)
}
