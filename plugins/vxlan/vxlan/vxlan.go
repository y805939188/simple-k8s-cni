package vxlan

import (
	"errors"
	"fmt"
	"testcni/cni"
	"testcni/consts"
	"testcni/etcd"
	_etcd "testcni/etcd"
	"testcni/ipam"
	_ipam "testcni/ipam"
	"testcni/plugins/vxlan/watcher"
	"testcni/skel"
	"testcni/utils"
)

const MODE = consts.MODE_VXLAN

type VxlanCNI struct {
}

func (vx *VxlanCNI) GetMode() string {
	return MODE
}

func startWatchNodeChange(ipam *ipam.IpamService, etcd *etcd.EtcdClient) error {
	// 如果这个默认端口已经正在使用了, 则认为之前已经有 pod 在在调用 cni 时启动过监听进程了, 这里可直接跳过
	pidInt, pidStr, err := utils.GetPidByPort(consts.DEFAULT_TMP_PORT)
	if err == nil && pidInt != -1 {
		// 说明 3190 这个端口正在被监听着, 就跳过 start watcher
		// 先看这个路径是否存在
		if utils.PathExists(consts.KUBE_TEST_CNI_TMP_DEAMON_DEFAULT_PATH) {
			// 如果该路径存在
			prevPidStr, err := utils.ReadContentFromFile(consts.KUBE_TEST_CNI_TMP_DEAMON_DEFAULT_PATH)
			// 尝试读出里头的 pid, 然后看这个 pid 和当前正在运行中的 port 对应的 pid 是否相等
			if err != nil || prevPidStr != pidStr {
				// 如果这个文件有问题的话, 就删掉文件重新把 pid 存进入
				utils.DeleteFile(consts.KUBE_TEST_CNI_TMP_DEAMON_DEFAULT_PATH)
				utils.CreateFile(consts.KUBE_TEST_CNI_TMP_DEAMON_DEFAULT_PATH, ([]byte)(pidStr), 0766)
			}
			return nil
		}
		// 如果该路径不存在文件的话, 可能是谁手贱给删了, 那就再创建一份儿
		utils.CreateFile(consts.KUBE_TEST_CNI_TMP_DEAMON_DEFAULT_PATH, ([]byte)(pidStr), 0766)
		return nil
	}
	// 走到这里说明还没有一条子进程能监听 etcd 中 node 上的 pod ip 的变换
	// 这里就启动监听
	return watcher.StartMapWatcher(ipam, etcd)
}

func initEverClient(args *skel.CmdArgs, pluginConfig *cni.PluginConf) (*ipam.IpamService, *etcd.EtcdClient, error) {
	_ipam.Init(pluginConfig.Subnet, "16", "32")
	ipam, err := _ipam.GetIpamService()
	if err != nil {
		return nil, nil, errors.New(fmt.Sprintln("初始化 ipam 客户端失败: %s", err.Error()))
	}
	_etcd.Init()
	etcd, err := _etcd.GetEtcdClient()
	if err != nil {
		return nil, nil, errors.New(fmt.Sprintln("初始化 etcd 客户端失败: %s", err.Error()))
	}
	return ipam, etcd, nil
}

/**
 * pluginConfig:
 * {
 *   "cniVersion": "0.3.0",
 *   "name": "testcni",
 *   "type": "testcni",
 *   "mode": "vxlan",
 *   "subnet": "10.244.0.0"
 * }
 */
func (vx *VxlanCNI) Bootstrap(args *skel.CmdArgs, pluginConfig *cni.PluginConf) (*cni.CNIResult, error) {
	utils.WriteLog("进到了 vxlan 模式了")

	// 0. 先把各种能用的上的客户端初始化咯
	ipam, etcd, err := initEverClient(args, pluginConfig)
	if err != nil {
		return nil, err
	}

	// 1. 开始监听 etcd 中 pod 和 subnet map 的变化, 注意该行为只能有一次
	err = startWatchNodeChange(ipam, etcd)
	if err != nil {
		return nil, err
	}

	// 2. 创建一对 veth pair 设备 veth_host 和 veth_net 作为默认网关

	// 3. 给这对儿网关 veth 设备中的 veth_host 加上 ip/32

	// 4. 创建一对儿 veth pair 作为 pod 的 veth

	// 5. 将 veth pair 设备加入到 kubelet 传来的 ns 下

	// 6. 给 ns 中的 veth 创建 ip/32, etcd 会自动通知其他 node

	// 7. 给这个 ns 中创建默认的路由表以及 arp 表, 让其能把流量都走到 ns 外

	// 8. 将 veth pair 的信息写入到 LXC_MAP_DEFAULT_PATH

	// 9. 将 veth pair 的 ip 与 node ip 的映射写入到 NODE_LOCAL_MAP_DEFAULT_PATH

	// 10. 给 veth pair 中留在 host 上的那半拉的 tc 打上 ingress 和 egress

	// 11. 创建一块儿 vxlan 设备

	// 12. 给这块儿 vxlan 设备的 tc 打上 ingress 和 egress

	return nil, errors.New("tmp error")
}

func (hostGW *VxlanCNI) Unmount(
	args *skel.CmdArgs,
	pluginConfig *cni.PluginConf,
) error {
	// TODO
	return nil
}

func (hostGW *VxlanCNI) Check(
	args *skel.CmdArgs,
	pluginConfig *cni.PluginConf,
) error {
	// TODO
	return nil
}

func init() {
	VxlanCNI := &VxlanCNI{}
	manager := cni.GetCNIManager()
	err := manager.Register(VxlanCNI)
	utils.WriteLog("即将注册 vxlan 模式 cni")
	if err != nil {
		utils.WriteLog("注册 vxlan cni 失败: ", err.Error())
		panic(err.Error())
	}
	utils.WriteLog("注册 vxlan 模式 cni 成功")
}
