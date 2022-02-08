package unilog

import (
	"fmt"
	"testing"
)

func TestGetHostByCode(t *testing.T) {
	appHostGlobal.add("comment", "192.168.1.1", 1)
	appHostGlobal.add("comment", "192.168.1.2", 2)
	appHostGlobal.add("comment", "192.168.1.3", 3)
	fmt.Println(GetHostByCode("comment", 5))
	fmt.Println(GetHostByCode("comment", 2))
	fmt.Println(GetHostByCode("comment", 3))
}
