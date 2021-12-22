package ipam

import (
	"fmt"
	"testing"
)

func TestIpam(t *testing.T) {

	Init("10.244.0.0", "16")
	is, err := GetIpamService()
	if err != nil {
		fmt.Println("ipam 初始化失败: ", err.Error())
		return
	}

	fmt.Println("成功: ", is.MaskIP)

	names, err := is.Get().NodeNames()
	if err != nil {
		fmt.Println("这里的 err 是: ", err.Error())
		return
	}

	for _, name := range names {
		fmt.Println("这里的 name 是: ", name)
		ip, err := is.Get().NodeIp(name)
		if err != nil {
			fmt.Println("这里的 err 是: ", err.Error())
			return
		}
		fmt.Println("这里的 ip 是: ", ip)
	}

}
