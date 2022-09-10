package ipam

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIpam(t *testing.T) {
	test := assert.New(t)

	clear := Init("10.244.0.0", "16", "32")
	is, err := GetIpamService()
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

	/********** test get **********/
	hostNetwork, err := is.Get().HostNetwork()
	test.Nil(err)
	fmt.Println(hostNetwork)

	// 获取主机对外网卡的 ip
	hostIp, err := is.Get().NodeIp(hostName)
	test.Nil(err)

	names, err := is.Get().NodeNames()
	test.Nil(err)
	test.Len(names, 3)

	networks, err := is.Get().AllHostNetwork()
	test.Nil(err)
	cidr, err := is.Get().CIDR(hostName)
	fmt.Println("主机的 cidr 是: ", cidr)
	test.Nil(err)
	for _, network := range networks {
		fmt.Println("节点 ", network.Name, " 的 ip 是: ", network.IP)
		if network.IP == hostIp {
			test.Equal(cidr, network.CIDR)
		}
	}

	/********** test set **********/
	ip := strings.Split(is.CurrentHostNetwork, ".")
	ip[3] = "10"
	is.Set().IPs(strings.Join(ip, "."))
	ip[3] = "99"
	is.Set().IPs(strings.Join(ip, "."))

	ips, err := is.Get().AllUsedIPs()
	test.Nil(err)
	test.Len(ips, 2)

	/********** test release **********/
	err = is.Release().IPs(strings.Join(ip, "."))
	test.Nil(err)
	ips, err = is.Get().AllUsedIPs()
	test.Nil(err)
	test.Len(ips, 1)

	err = clear()
	test.Nil(err)
}
