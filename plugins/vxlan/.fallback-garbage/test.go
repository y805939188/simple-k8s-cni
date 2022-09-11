package main

import (
	"fmt"
	"os"
	"unsafe"

	"github.com/cilium/ebpf"
	// "github.com/cilium/ebpf/internal/unix"
)

const (
	MapName    = "ding_lxc"
	MaxEntries = 65535
	PortMapMax = 16
)

type EndpointKey struct {
	IP uint32
}

type EndpointInfo struct {
	IfIndex uint32
	LxcID   uint16
	_       uint16
	MAC     uint64
	NodeMAC uint64
}

func PathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func CreateMapWithPin(pinPath string) *ebpf.Map {
	spec := ebpf.MapSpec{
		Name:       MapName,
		Type:       ebpf.Hash,
		KeySize:    uint32(unsafe.Sizeof(EndpointKey{})),
		ValueSize:  uint32(unsafe.Sizeof(EndpointInfo{})),
		MaxEntries: MaxEntries,
		Flags:      0,
	}
	m1, err := ebpf.NewMap(&spec)
	if err != nil {
		panic(err)
	}
	err = m1.Pin(pinPath)
	if err != nil {
		panic(err)
	}
	return m1
}

func main() {
	// sys.MapGetFdById()
	// sys.Get
	// fmt.Println(111, uint32(unsafe.Sizeof(EndpointKey{})))
	// fmt.Println(222, uint32(unsafe.Sizeof(EndpointInfo{})))
	// testErr := unix.Close(45)
	// if testErr != nil {
	// 	fmt.Println(testErr.Error())
	// }
	// return
	testPath := "/sys/fs/bpf/tc/globals/ding_lxc"
	var m *ebpf.Map

	m, _ = ebpf.LoadPinnedMap("/sys/fs/bpf/tc/globals/ding_test", &ebpf.LoadPinOptions{})
	// m.Unpin()
	// m.Close()
	// m.Freeze()
	// m.
	return

	var err error
	if !PathExists(testPath) {
		m = CreateMapWithPin(testPath)
	} else {
		fmt.Println("已经存在")
		m, err = ebpf.LoadPinnedMap(testPath, &ebpf.LoadPinOptions{})
		if err != nil {
			panic(err)
		}
		// fmt.Println(m.Close())
	}
	if m == nil {
		fmt.Println("创建 map 失败")
	}
	// return
	// fmt.Println("m 的 fd 是: ", m.FD())
	err = m.Put(EndpointKey{IP: 6}, EndpointInfo{
		IfIndex: 2,
		LxcID:   3,
		MAC:     4,
		NodeMAC: 5,
	})
	if err != nil {
		panic(err)
	}
}
