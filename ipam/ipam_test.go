package ipam

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIpam(t *testing.T) {
	test := assert.New(t)

	clear := Init("10.244.0.0", "16", "32")
	err := clear()

	is, err := GetIpamService()
	return
	if err != nil {
		fmt.Println("ipam 初始化失败: ", err.Error())
		return
	}

	test.Equal(is.Subnet, "10.244.0.0")
	test.Equal(is.MaskSegment, "16")
	test.Equal(is.MaskIP, "255.255.0.0")
	test.Equal(is.PodMaskSegment, "32")
	test.Equal(is.PodMaskIP, "255.255.255.255")
	hostName, err := os.Hostname()
	if err != nil {
		panic(err.Error())
	}

	hostNetwork, err := is.Get().HostNetwork()
	test.Nil(err)
	fmt.Println(hostNetwork)

	// 获取主机对外网卡的 ip
	hostIp, err := is.Get().NodeIp(hostName)
	test.Nil(err)
	fmt.Println(hostIp)

	// test.Equal(is.MaskIP, "255.255.0.0")

	// fmt.Println("成功: ", is.MaskIP)
	// test.Equal(is.MaskIP, "255.255.0.0")
	// cidr, _ := is.Get().CIDR("ding-net-master")
	// test.Equal(cidr, "10.244.0.0/24")
	// cidr, _ = is.Get().CIDR("ding-net-node-1")
	// test.Equal(cidr, "10.244.1.0/24")
	// cidr, _ = is.Get().CIDR("ding-net-node-2")
	// test.Equal(cidr, "10.244.2.0/24")

	// names, err := is.Get().NodeNames()
	// if err != nil {
	// 	fmt.Println("这里的 err 是: ", err.Error())
	// 	return
	// }

	// test.Equal(len(names), 3)

	// for _, name := range names {
	// 	ip, err := is.Get().NodeIp(name)
	// 	if err != nil {
	// 		fmt.Println("这里的 err 是: ", err.Error())
	// 		return
	// 	}
	// 	fmt.Println("这里的 ip 是: ", ip)
	// }

	// nets, err := is.Get().AllHostNetwork()
	// if err != nil {
	// 	fmt.Println("这里的 err 是: ", err.Error())
	// 	return
	// }
	// fmt.Println("集群全部网络信息是: ", nets)

	// for _, net := range nets {
	// 	fmt.Println("其他主机的网络信息是: ", net)
	// }

	// currentNet, err := is.Get().HostNetwork()
	// if err != nil {
	// 	fmt.Println("这里的 err 是: ", err.Error())
	// 	return
	// }
	// fmt.Println("本机的网络信息是: ", currentNet)
}
