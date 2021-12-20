package main

import (
	"crypto/rand"
	"fmt"
	"os"

	"github.com/vishvananda/netlink"
)

func RandomVethName() (string, error) {
	entropy := make([]byte, 4)
	fmt.Println(999, entropy)
	_, err := rand.Read(entropy)
	if err != nil {
		return "", fmt.Errorf("failed to generate random veth name: %v", err)
	}

	// NetworkManager (recent versions) will ignore veth devices that start with "veth"
	return fmt.Sprintf("veth%x", entropy), nil
}

func main() {
	// val, err := RandomVethName()
	// if err != nil {
	// 	fmt.Println("错误是: ", err.Error())
	// }
	// fmt.Println("这里的值是: ", val)
	l, err := netlink.LinkByName("ens33")
	if err != nil && !os.IsExist(err) {
		fmt.Println("获取 ens33 出错, err: ", err.Error())
	} else {
		fmt.Println(l)
	}
}
