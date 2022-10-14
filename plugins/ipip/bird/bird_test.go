package bird

import (
	"fmt"
	"testcni/ipam"
	"testcni/utils"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBird(t *testing.T) {
	test := assert.New(t)
	ipam.Init("10.244.0.0/16", nil)
	is, err := ipam.GetIpamService()
	if err != nil {
		fmt.Println("ipam 初始化失败: ", err.Error())
		return
	}
	// fmt.Println(is.Get().CIDR("cni-test-1"))
	// fmt.Println(is.Get().Gateway())
	// fmt.Println(is.Get().CurrentSubnet())
	// return

	config, err := GenConfig(is)
	test.Nil(err)
	fmt.Println(config)

	utils.DeleteFile("/opt/testcni/bird.cfg")
	err = GenConfigFile(is)
	test.Nil(err)
	test.True(utils.FileIsExisted("/opt/testcni/bird.cfg"))
	str, err := utils.ReadContentFromFile("/opt/testcni/bird.cfg")
	test.Nil(err)
	test.Equal(str, config)
	// return
	// utils.DeleteFile("/opt/testcni/bird.cfg")
	pid, err := StartBirdDaemon("/opt/testcni/bird.cfg")
	test.Nil(err)
	fmt.Println("bird 的子进程 pid 是: ", pid)
}
