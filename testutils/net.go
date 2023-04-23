package testutils

import (
	"fmt"
	"net"
)

func PickUnusedHostPort() (string, error) {
	if listener, err := net.Listen("tcp", "localhost:0"); err != nil {
		return "", fmt.Errorf("failed to pick unused local host:port: %s", err)
	} else {
		listener.Close()
		return listener.Addr().String(), nil
	}
}
