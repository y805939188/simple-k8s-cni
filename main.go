package main

import (
	"errors"
	"fmt"
	"testcni/cni"
	"testcni/helper"

	_ "testcni/plugins/hostgw"
	_ "testcni/plugins/ipip"
	_ "testcni/plugins/vxlan/vxlan"
	_ "testcni/plugins/xvlan/ipvlan"
	_ "testcni/plugins/xvlan/macvlan"
	"testcni/skel"
	"testcni/utils"

	"github.com/containernetworking/cni/pkg/version"
	bv "github.com/containernetworking/plugins/pkg/utils/buildversion"
)

func cmdAdd(args *skel.CmdArgs) error {
	utils.WriteLog("进入到 cmdAdd")
	helper.TmpLogArgs(args)

	// 从 args 里把 config 给捞出来
	pluginConfig := helper.GetConfigs(args)
	if pluginConfig == nil {
		errMsg := fmt.Sprintf("add: 从 args 中获取 plugin config 失败, config: %s", string(args.StdinData))
		utils.WriteLog(errMsg)
		return errors.New(errMsg)
	}

	mode, cniVersion := helper.GetBaseInfo(pluginConfig)
	if pluginConfig.CNIVersion == "" {
		pluginConfig.CNIVersion = cniVersion
	}

	// 将 args 和 configs 以及要使用的插件模式都传给 cni manager
	cniManager := cni.
		GetCNIManager().
		SetBootstrapConfigs(pluginConfig).
		SetBootstrapArgs(args).
		SetBootstrapCNIMode(mode)
	if cniManager == nil {
		utils.WriteLog("cni 插件未初始化完成")
		return errors.New("cni plugins register failed")
	}

	// 启动对应 mode 的插件开始设置乱七八糟的网卡等
	err := cniManager.BootstrapCNI()
	if err != nil {
		utils.WriteLog("设置 cni 失败: ", err.Error())
		return err
	}

	// 将结果打印到标准输出
	err = cniManager.PrintResult()
	if err != nil {
		utils.WriteLog("打印 cni 执行结果失败: ", err.Error())
		return err
	}
	return nil
}

func cmdDel(args *skel.CmdArgs) error {
	utils.WriteLog("进入到 cmdDel")
	helper.TmpLogArgs(args)

	pluginConfig := helper.GetConfigs(args)
	if pluginConfig == nil {
		errMsg := fmt.Sprintf("del: 从 args 中获取 plugin config 失败, config: %s", string(args.StdinData))
		utils.WriteLog(errMsg)
		return errors.New(errMsg)
	}
	mode, _ := helper.GetBaseInfo(pluginConfig)

	cniManager := cni.
		GetCNIManager().
		SetUnmountConfigs(pluginConfig).
		SetUnmountArgs(args).
		SetUnmountCNIMode(mode)

	// 这里的 del 如果返回 error 的话, kubelet 就会尝试一直不停地执行 StopPodSandbox
	// 直到删除后的 error 返回 nil 未知
	// return errors.New("test cmdDel")
	return cniManager.UnmountCNI()
}

func cmdCheck(args *skel.CmdArgs) error {
	utils.WriteLog("进入到 cmdCheck")
	helper.TmpLogArgs(args)

	pluginConfig := helper.GetConfigs(args)
	if pluginConfig == nil {
		errMsg := fmt.Sprintf("check: 从 args 中获取 plugin config 失败, config: %s", string(args.StdinData))
		utils.WriteLog(errMsg)
		return errors.New(errMsg)
	}
	mode, _ := helper.GetBaseInfo(pluginConfig)

	cniManager := cni.
		GetCNIManager().
		SetCheckConfigs(pluginConfig).
		SetCheckArgs(args).
		SetCheckCNIMode(mode)
	return cniManager.CheckCNI()
}

func main() {
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, bv.BuildString("testcni"))
}
