package testutils

import (
	"errors"
	"net"
)

func PickUnusedHostPort() (string, error) {
	if listener, err := net.Listen("tcp", "localhost:0"); err != nil {
		return "", errors.New("failed to pick unused endpoint: " + err.Error())
	} else {
		listener.Close()
		return listener.Addr().String(), nil
	}
}
