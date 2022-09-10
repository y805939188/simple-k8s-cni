package helper

import (
	"encoding/json"
	"testcni/cni"
	"testcni/consts"
	"testcni/skel"
	"testcni/utils"
)

func GetConfigs(args *skel.CmdArgs) *cni.PluginConf {
	pluginConfig := &cni.PluginConf{}
	if err := json.Unmarshal(args.StdinData, pluginConfig); err != nil {
		utils.WriteLog("args.StdinData 转 pluginConfig 失败")
		return nil
	}
	utils.WriteLog("这里的结果是: pluginConfig.Bridge", pluginConfig.Bridge)
	utils.WriteLog("这里的结果是: pluginConfig.CNIVersion", pluginConfig.CNIVersion)
	utils.WriteLog("这里的结果是: pluginConfig.Name", pluginConfig.Name)
	utils.WriteLog("这里的结果是: pluginConfig.Subnet", pluginConfig.Subnet)
	utils.WriteLog("这里的结果是: pluginConfig.Type", pluginConfig.Type)
	utils.WriteLog("这里的结果是: pluginConfig.Mode", pluginConfig.Mode)
	return pluginConfig
}

func GetBaseInfo(plugin *cni.PluginConf) (mode string, cniVersion string) {
	mode = plugin.Mode
	if mode == "" {
		mode = consts.MODE_HOST_GW
	}
	cniVersion = plugin.CNIVersion
	if cniVersion == "" {
		cniVersion = "0.3.0"
	}
	return mode, cniVersion
}

func TmpLogArgs(args *skel.CmdArgs) {
	utils.WriteLog(
		"这里的 CmdArgs 是: ", "ContainerID: ", args.ContainerID,
		"Netns: ", args.Netns,
		"IfName: ", args.IfName,
		"Args: ", args.Args,
		"Path: ", args.Path,
		"StdinData: ", string(args.StdinData),
	)
}
