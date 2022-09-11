package utils

import (
	"os"
	"os/exec"
	"syscall"
)

func Bash(cmd string) (out string, exitcode int) {
	cmdobj := exec.Command("bash", "-c", cmd)
	output, err := cmdobj.CombinedOutput()
	if err != nil {
		// Get the exitcode of the output
		if ins, ok := err.(*exec.ExitError); ok {
			out = string(output)
			exitcode = ins.ExitCode()
			return out, exitcode
		}
	}
	return string(output), 0
}

func ExecCommand(name string, cmdParams ...string) (int, error) {
	cmd := exec.Command(name, cmdParams...)
	cmd.Stdout = os.Stdout
	err := cmd.Start()
	if err != nil {
		return -1, err
	}
	return cmd.Process.Pid, nil
}

func KillByPID(pid int) error {
	err := syscall.Kill(pid, syscall.SIGINT)
	if err != nil {
		return err
	}
	return nil
}
