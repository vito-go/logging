//go:build go1.17
// +build go1.17

package tid

import (
	"errors"
	"net"
)

func getPrivateIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.IsPrivate() {
			return ipNet.IP.String(), err
		}
	}
	return "", errors.New("no private ip")
}
