package main

import (
	"encoding/json"
	"fmt"
	"testcni/ipam"
	"testcni/net"
	"testcni/skel"
	"testcni/utils"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ns"
	bv "github.com/containernetworking/plugins/pkg/utils/buildversion"
)

type PluginConf struct {
	// NetConf 里头指定了一个 plugin 的最基本的信息, 比如 CNIVersion, Name, Type 等, 当然还有在 containerd 中塞进来的 PrevResult
	types.NetConf

	// 这个 runtimeConfig 是可以在 /etc/cni/net.d/xxx.conf 中配置一个
	// 类似 "capabilities": {"xxx": true, "yyy": false} 这样的属性
	// 表示说要在运行时开启 xxx 的能力, 不开启 yyy 的能力
	// 然后等容器跑起来之后(或者被拉起来之前)可以直接通过设置环境变量 export CAP_ARGS='{ "xxx": "aaaa", "yyy": "bbbb" }'
	// 来开启或关闭某些能力
	// 然后通过 stdin 标准输入读进来的数据中就会多出一个 RuntimeConfig 属性, 里面就是 runtimeConfig: { "xxx": "aaaa" }
	// 因为 yyy 在 /etc/cni/net.d/xxx.conf 中被设置为了 false
	// 官方使用范例: https://kubernetes.feisky.xyz/extension/network/cni
	// cni 源码中实现: /cni/libcni/api.go:injectRuntimeConfig
	RuntimeConfig *struct {
		TestConfig map[string]interface{} `json:"testConfig"`
	} `json:"runtimeConfig"`

	// 这里可以自由定义自己的 plugin 中配置了的参数然后自由处理
	Bridge string `json:"bridge"`
	Subnet string `json:"subnet"`
}

func cmdAdd(args *skel.CmdArgs) error {
	utils.WriteLog("进入到 cmdAdd")
	utils.WriteLog(
		"这里的 CmdArgs 是: ", "ContainerID: ", args.ContainerID,
		"Netns: ", args.Netns,
		"IfName: ", args.IfName,
		"Args: ", args.Args,
		"Path: ", args.Path,
		"StdinData: ", string(args.StdinData))

	pluginConfig := &PluginConf{}
	if err := json.Unmarshal(args.StdinData, pluginConfig); err != nil {
		utils.WriteLog("args.StdinData 转 pluginConfig 失败")
		return err
	}
	// utils.WriteLog("这里的结果是: pluginConfig.Bridge", pluginConfig.Bridge)
	// utils.WriteLog("这里的结果是: pluginConfig.CNIVersion", pluginConfig.CNIVersion)
	// utils.WriteLog("这里的结果是: pluginConfig.Name", pluginConfig.Name)
	// utils.WriteLog("这里的结果是: pluginConfig.Subnet", pluginConfig.Subnet)
	// utils.WriteLog("这里的结果是: pluginConfig.Type", pluginConfig.Type)
	// return errors.New("test cmdAdd")

	// 使用 kubelet(containerd) 传过来的 subnet 地址初始化 ipam
	ipam.Init(pluginConfig.Subnet)
	ipamClient, err := ipam.GetIpamService()
	if err != nil {
		utils.WriteLog("创建 ipam 客户端出错, err: ", err.Error())
		return err
	}

	// 从 ipam 中拿到一个未使用的 ip 地址
	podIP, err := ipamClient.Get().UnusedIP()
	if err != nil {
		utils.WriteLog("获取 podIP 出错, err: ", err.Error())
		return err
	}

	utils.WriteLog("这里获取到的 podIP 是: ", podIP)

	gateway, err := ipamClient.Get().Gateway()
	if err != nil {
		utils.WriteLog("获取 gateway 出错, err: ", err.Error())
		return err
	}
	utils.WriteLog("这里获取到的 gateway 是: ", gateway)

	// 1. 创建网桥
	// 获取网桥名字
	bridgeName := pluginConfig.Bridge
	if bridgeName != "" {
		bridgeName = "testcni0"
	}
	// 这里如果不同节点间通信的方式使用 vxlan 的话, 这里需要变成 1460
	// 因为 vxlan 设备会给报头中加一个 40 字节的 vxlan 头部
	mtu := 1500 // 1460

	// br, err := net.CreateBridge(bridgeName,  gateway, mtu)
	cidr := gateway + "/" + ipamClient.Mask
	br, err := net.CreateBridge(bridgeName, cidr, mtu)
	if err != nil {
		utils.WriteLog("创建网卡失败, err: ", err.Error())
		return err
	}
	// // 把这个地址设置到 etcd 中
	// ipamClient.Set().IPs(podIP)
	// fmt.Println("创建出来的 bridge 名字是: ", br.Attrs().Name)
	// fmt.Println("创建出来的 bridge mac 是: ", br.Attrs().HardwareAddr.String())
	utils.WriteLog("创建出来的 bridge 名字是: ", br.Attrs().Name)
	utils.WriteLog("创建出来的 bridge mac 是: ", br.Attrs().HardwareAddr.String())
	// 2. 创建 veth pair
	podEthName := args.IfName          // pod 内的网卡名字
	netns, err := ns.GetNS(args.Netns) // pod 的 net ns
	// if err != nil {
	// 	return err
	// }
	// 3. 将两个 veth 一个查到 pod 的 netns, 一个查到网桥

	return nil
}

func cmdDel(args *skel.CmdArgs) error {
	utils.WriteLog("进入到 cmdDel")
	utils.WriteLog(
		"这里的 CmdArgs 是: ", "ContainerID: ", args.ContainerID,
		"Netns: ", args.Netns,
		"IfName: ", args.IfName,
		"Args: ", args.Args,
		"Path: ", args.Path,
		"StdinData: ", string(args.StdinData))
	// 这里的 del 如果返回 error 的话, kubelet 就会尝试一直不停地执行 StopPodSandbox
	// 直到删除后的 error 返回 nil 未知
	// return errors.New("test cmdDel")
	return nil
}

func _test_clear_etcd() {
	// ipam.Init("192.168.0.0", "16")
	ipam.Init("10.244.0.0", "16")
	is, err := ipam.GetIpamService()
	if err != nil {
		fmt.Println("ipam 初始化失败: ", err.Error())
		return
	}
	err = is.EtcdClient.Del("/testcni/ipam/10.244.0.0/16/ding-net-master")
	if err != nil {
		fmt.Println("删除 /testcni/ipam/10.244.0.0/16/ding-net-master 失败: ", err.Error())
		return
	}

	err = is.EtcdClient.Del("/testcni/ipam/10.244.0.0/16/pool")
	if err != nil {
		fmt.Println("删除 /testcni/ipam/10.244.0.0/16/pool 失败: ", err.Error())
		return
	}

	err = is.EtcdClient.Del("/testcni/ipam/testcni/ipam/10.244.0.0/16/ding-net-master/10.244.0.0")
	if err != nil {
		fmt.Println("删除 /testcni/ipam/testcni/ipam/10.244.0.0/16/ding-net-master/10.244.0.0 失败: ", err.Error())
		return
	}

	// err = is.EtcdClient.Del("/testcni/ipam/192.168.0.0/16/ding-net-master")
	// if err != nil {
	// 	fmt.Println("删除 /testcni/ipam/192.168.0.0/16/ding-net-master 失败: ", err.Error())
	// 	return
	// }
	// err = is.EtcdClient.Del("/testcni/ipam/192.168.0.0/16/pool")
	// if err != nil {
	// 	fmt.Println("/testcni/ipam/192.168.0.0/16/pool 失败: ", err.Error())
	// 	return
	// }
	// err = is.EtcdClient.Del("/testcni/ipam/192.168.0.0/16/ding-net-node-1")
	// if err != nil {
	// 	fmt.Println("删除 /testcni/ipam/192.168.0.0/16/ding-net-node-1 失败: ", err.Error())
	// 	return
	// }
	// err = is.EtcdClient.Del("/testcni/ipam/testcni/ipam/192.168.0.0/16/ding-net-master/192.168.0.0")
	// if err != nil {
	// 	fmt.Println("/testcni/ipam/testcni/ipam/192.168.0.0/16/ding-net-master/192.168.0.0 失败: ", err.Error())
	// 	return
	// }
	// err = is.EtcdClient.Del("/testcni/ipam/testcni/ipam/192.168.0.0/16/ding-net-node-1/192.168.1.0")
	// if err != nil {
	// 	fmt.Println("/testcni/ipam/testcni/ipam/192.168.0.0/16/ding-net-node-1/192.168.1.0 失败: ", err.Error())
	// 	return
	// }
}

func cmdCheck(args *skel.CmdArgs) error {
	utils.WriteLog("进入到 cmdCheck")
	utils.WriteLog(
		"这里的 CmdArgs 是: ", "ContainerID: ", args.ContainerID,
		"Netns: ", args.Netns,
		"IfName: ", args.IfName,
		"Args: ", args.Args,
		"Path: ", args.Path,
		"StdinData: ", string(args.StdinData))
	return nil
}

func main() {
	_test_clear_etcd()
	return

	// eee, _ := os.Hostname()
	// fmt.Println("这里的主机名是: ", eee)
	// PluginMain 里头会 case "ADD" "DEL" 等
	// PluginMain 的第一步一定是先传过来 VERSION 命令
	// 在 version.All 中预设了几个 versions
	// var All = PluginSupports("0.1.0", "0.2.0", "0.3.0", "0.3.1", "0.4.0", "1.0.0")
	// 在 /etc/cni/net.d 中的 cniVersion 必须要和其中的某一个保持一致
	// 否则的话 kubelet(containerd) 会一直发 VERSION 指令过来

	// ipam.Init("192.168.0.0", "16")
	// ipamClient, err := ipam.GetIpamService()
	// if err != nil {
	// 	fmt.Println("ipam 初始化失败: ", err.Error())
	// 	return
	// }

	// ips, err := ipamClient.Get().AllUsedIPs()
	// // fmt.Println("din1: ", ipamClient.Get)
	// // ip, err := ipamClient.Get().UnusedIP()
	// if err != nil {
	// 	fmt.Println("获取 ip 失败: ", err.Error())
	// 	return
	// }
	// fmt.Println(111, ips)

	// ip, err := ipamClient.Get().UnusedIP()
	// fmt.Println("这里的 next ip 是: ", ip)
	// if err != nil {
	// 	fmt.Println("获取 ip 失败: ", err.Error())
	// 	return
	// }

	// ipamClient.Set().IPs(ip)
	// ip, err = ipamClient.Get().UnusedIP()
	// fmt.Println("这里的 next ip 是: ", ip)
	// if err != nil {
	// 	fmt.Println("获取 ip 失败: ", err.Error())
	// 	return
	// }
	// ipamClient.Set().IPs(ip)
	// ip, err = ipamClient.Get().UnusedIP()
	// fmt.Println("这里的 next ip 是: ", ip)
	// if err != nil {
	// 	fmt.Println("获取 ip 失败: ", err.Error())
	// 	return
	// }
	// ipamClient.Set().IPs(ip)
	// ips, err = ipamClient.Get().AllUsedIPs()
	// // fmt.Println("din1: ", ipamClient.Get)
	// // ip, err := ipamClient.Get().UnusedIP()
	// if err != nil {
	// 	fmt.Println("获取 ip 失败: ", err.Error())
	// 	return
	// }
	// fmt.Println(111, ips)

	// _test_clear_etcd()
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, bv.BuildString("testcni"))
}
