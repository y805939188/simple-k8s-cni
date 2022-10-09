package bird

import (
	"fmt"
	"os"
	"os/exec"
	"testcni/utils"
)

func StartBirdDaemon(configPath string) (int, error) {
	if !utils.FileIsExisted(configPath) {
		return -1, fmt.Errorf("the config path %s not exist", configPath)
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
	return cmd.Process.Pid, nil
}
