package vxlan

import (
	"testcni/skel"
	"testcni/struct/plugins"
	"testcni/utils"
)

type VxlanMode struct {
}

func (vx *VxlanMode) SetupVxlanMode(args *skel.CmdArgs, pluginConfig *plugins.PluginConf) error {
	utils.WriteLog("进到了 vxlan 模式了")
	return nil
}
