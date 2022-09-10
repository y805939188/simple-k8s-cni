package vxlan

import (
	"errors"
	"testcni/cni"
	"testcni/consts"
	"testcni/skel"
	"testcni/utils"
)

const MODE = consts.MODE_VXLAN

type VxlanCNI struct {
}

func (vx *VxlanCNI) GetMode() string {
	return MODE
}

func (vx *VxlanCNI) Bootstrap(args *skel.CmdArgs, pluginConfig *cni.PluginConf) (*cni.CNIResult, error) {
	utils.WriteLog("进到了 vxlan 模式了")

	// 1. 创建一对儿 veth pair

	// 2. 将 veth pair 的信息写入到 LXC_MAP_DEFAULT_PATH

	// 3. 将 veth pair 的 ip 与 node ip 的映射写入到 NODE_LOCAL_MAP_DEFAULT_PATH

	// 3. 将 veth pair 的 ip 与 node ip 的映射同步到 etcd

	// 4. 创建一块儿 vxlan 设备

	// 5.

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
