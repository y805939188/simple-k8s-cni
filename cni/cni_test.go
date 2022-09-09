package cni

import (
	"errors"
	"testcni/skel"
	"testing"

	currentTypes "github.com/containernetworking/cni/pkg/types"
	types "github.com/containernetworking/cni/pkg/types/100"
	"github.com/stretchr/testify/assert"
)

type tmpcni struct{}

const TEST_MODE = "ding-test"

var Err_TEST_ERROR = errors.New("this is test err")

var TEST_CNIResult = &CNIResult{CNIVersion: "dingdingdingding"}

func (tmp *tmpcni) GetMode() string {
	return TEST_MODE
}

func (tmp *tmpcni) Bootstrap(args *skel.CmdArgs, pluginConfig *PluginConf) (*CNIResult, error) {
	return TEST_CNIResult, nil
}

func (tmp *tmpcni) Unmount(args *skel.CmdArgs, pluginConfig *PluginConf) error {
	return Err_TEST_ERROR
}

func (tmp *tmpcni) Check(args *skel.CmdArgs, pluginConfig *PluginConf) error {
	return Err_TEST_ERROR
}

func TestCNI(t *testing.T) {
	test := assert.New(t)
	manager := GetCNIManager()

	/****** test init *******/
	test.NotNil(manager)
	test.Empty(manager.cniMap)
	test.Equal(manager.bootstrapMode, "")
	test.Nil(manager.bootstrapArgs)
	test.Nil(manager.bootstrapPluginConfig)
	test.Equal(manager.unmountMode, "")
	test.Nil(manager.unmountArgs)
	test.Nil(manager.unmountPluginConfig)
	test.Equal(manager.checkMode, "")
	test.Nil(manager.checkArgs)
	test.Nil(manager.checkPluginConfig)

	/****** test getter/setter *******/
	manager.SetBootstrapCNIMode(TEST_MODE)
	test.EqualValues(manager.getBootstrapMode(), TEST_MODE)
	manager.SetBootstrapArgs(&skel.CmdArgs{ContainerID: "ding11", Netns: "ding21"})
	test.EqualValues(manager.getBootstrapArgs(), &skel.CmdArgs{ContainerID: "ding11", Netns: "ding21"})
	manager.SetBootstrapConfigs(&PluginConf{Bridge: "ding11", Subnet: "ding21"})
	test.EqualValues(manager.getBootstrapConfigs(), &PluginConf{Bridge: "ding11", Subnet: "ding21"})

	manager.SetUnmountCNIMode(TEST_MODE)
	test.EqualValues(manager.getUnmountMode(), TEST_MODE)
	manager.SetUnmountArgs(&skel.CmdArgs{ContainerID: "ding12", Netns: "ding22"})
	test.EqualValues(manager.getUnmountArgs(), &skel.CmdArgs{ContainerID: "ding12", Netns: "ding22"})
	manager.SetUnmountConfigs(&PluginConf{Bridge: "ding12", Subnet: "ding22"})
	test.EqualValues(manager.getUnmountConfigs(), &PluginConf{Bridge: "ding12", Subnet: "ding22"})

	manager.SetCheckCNIMode(TEST_MODE)
	test.EqualValues(manager.getCheckMode(), TEST_MODE)
	manager.SetCheckArgs(&skel.CmdArgs{ContainerID: "ding13", Netns: "ding23"})
	test.EqualValues(manager.getCheckArgs(), &skel.CmdArgs{ContainerID: "ding13", Netns: "ding23"})
	manager.SetCheckConfigs(&PluginConf{Bridge: "ding13", Subnet: "ding23"})
	test.EqualValues(manager.getCheckConfigs(), &PluginConf{Bridge: "ding13", Subnet: "ding23"})

	/****** test Register *******/
	var _ CNI = (*tmpcni)(nil)
	tmp := tmpcni{}
	err := manager.Register(&tmp)
	test.Nil(err)
	test.Equal(len(manager.cniMap), 1)
	cni := manager.getCNI(TEST_MODE)
	test.NotNil(cni)

	/****** test plugin bootstrap/unmount/check *******/
	tmpRes, err := cni.Bootstrap(nil, nil)
	test.Nil(err)
	test.EqualValues(tmpRes, TEST_CNIResult)
	err = cni.Unmount(nil, nil)
	test.ErrorIs(err, Err_TEST_ERROR)
	err = cni.Check(nil, nil)
	test.ErrorIs(err, Err_TEST_ERROR)

	err = manager.BootstrapCNI()
	test.Nil(err)
	err = manager.UnmountCNI()
	test.ErrorIs(err, Err_TEST_ERROR)
	err = manager.CheckCNI()
	test.ErrorIs(err, Err_TEST_ERROR)

	/****** test PrintResult *******/
	tmpPluginConfig := &PluginConf{Bridge: "ding11", Subnet: "ding21"}
	tmpPluginConfig.CNIVersion = "ding666"
	manager.SetBootstrapConfigs(tmpPluginConfig)
	err = manager.PrintResult()
	test.Nil(err)

	/****** test transformCNIResultToTypes100 *******/
	testCNIResult := &CNIResult{
		CNIVersion: "ding1",
		Interfaces: []*Interface{{Name: "ding1", Mac: "ding3", Sandbox: "ding4"}},
		DNS:        DNS{Nameservers: []string{"ding5"}},
	}

	testRealCNIResult := &types.Result{
		CNIVersion: "ding1",
		Interfaces: []*types.Interface{{Name: "ding1", Mac: "ding3", Sandbox: "ding4"}},
		DNS:        currentTypes.DNS{Nameservers: []string{"ding5"}},
	}
	tmpRealCNIResult, err := transformCNIResultToPrintTypes(testCNIResult)
	test.Nil(err)
	test.EqualValues(tmpRealCNIResult, testRealCNIResult)
}
