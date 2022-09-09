package main

import (
	"encoding/json"
	"testcni/cni"
	"testcni/consts"
	"testcni/skel"
	"testcni/utils"
)

func getConfigs(args *skel.CmdArgs) *cni.PluginConf {
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

func getBaseInfo(plugin *cni.PluginConf) (mode string, cniVersion string) {
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
