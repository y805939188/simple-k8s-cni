package main

import (
	"fmt"
	"testcni/utils"

	// "github.com/containernetworking/cni/pkg/skel"
	"testcni/etcd"
	"testcni/skel"

	"github.com/containernetworking/cni/pkg/types"
	// "fmt"
	// "io/ioutil" //io 工具包
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

	Bridge string `json:"bridge"` // 这里可以自由定义自己的 plugin 中配置了的参数然后自由处理
	// // Add plugin-specifc flags here
	// MyAwesomeFlag     bool   `json:"myAwesomeFlag"`
	// AnotherAwesomeArg string `json:"anotherAwesomeArg"`
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
	// return errors.New("test cmdAdd")
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
	// PluginMain 里头会 case "ADD" "DEL" 等
	// PluginMain 的第一步一定是先传过来 VERSION 命令
	// 在 version.All 中预设了几个 versions
	// var All = PluginSupports("0.1.0", "0.2.0", "0.3.0", "0.3.1", "0.4.0", "1.0.0")
	// 在 /etc/cni/net.d 中的 cniVersion 必须要和其中的某一个保持一致
	// 否则的话 kubelet(containerd) 会一直发 VERSION 指令过来
	etcd.Init()
	etcdClient, err := etcd.GetEtcdClient()

	if err != nil {
		fmt.Println("etcd 初始化失败: ", err.Error())
	}

	if etcdClient != nil {
		fmt.Println("etcd 的版本是: ", etcdClient.Version)
	}

	val, err := etcdClient.Get("/ding-test1")
	if err != nil {
		fmt.Println("etcd 获取失败: ", err.Error())
	} else {
		fmt.Println("获取成功: ", val)
	}

	etcdClient.Set("/ding-test1", "test222222")
	val, err = etcdClient.Get("/ding-test1")
	if err != nil {
		fmt.Println("etcd 获取失败2: ", err.Error())
	} else {
		fmt.Println("获取成功2: ", val)
	}
	// skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, bv.BuildString("testcni"))
}
