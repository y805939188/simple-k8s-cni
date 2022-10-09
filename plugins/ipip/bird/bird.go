package bird

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"testcni/consts"
	"testcni/utils"
)

func StartBirdDaemon(configPath string) (int, error) {
	if !utils.FileIsExisted(configPath) {
		return -1, fmt.Errorf("the config path %s not exist", configPath)
	}

	// 先看 bird deamon 这个路径是否存在
	if utils.PathExists(consts.KUBE_TEST_CNI_DEFAULT_BIRD_DEAMON_PATH) {
		// 如果该路径存在
		pid, err := utils.ReadContentFromFile(consts.KUBE_TEST_CNI_DEFAULT_BIRD_DEAMON_PATH)
		if err != nil {
			return -1, err
		}
		// 尝试读出里头的 pid, 然后看这个 pid 当前是不是真的在运行
		if utils.FileIsExisted(fmt.Sprintf("/proc/%s", pid)) {
			// 说明当前 host 上的 bird 正在运行可以直接返回
			return strconv.Atoi(pid)
		} else {
			// 说明当前 host 上的 bird 已经退出了, 那就删掉这个文件
			utils.DeleteFile(consts.KUBE_TEST_CNI_DEFAULT_BIRD_DEAMON_PATH)
		}
	}

	cmd := exec.Command(
		"/opt/testcni/bird",
		"-R",
		"-s",
		"/var/run/bird.ctl",
		"-d",
		"-c",
		configPath,
	)
	cmd.Stdout = os.Stdout
	err := cmd.Start()
	if err != nil {
		return -1, err
	}
	pid := strconv.Itoa(cmd.Process.Pid)
	utils.CreateFile(consts.KUBE_TEST_CNI_DEFAULT_BIRD_DEAMON_PATH, ([]byte)(pid), 0766)
	return cmd.Process.Pid, nil
}
