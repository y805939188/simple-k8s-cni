package tc

import (
	"fmt"
	"os/exec"
)

// TODO: 可以尝试换成 go-tc
func _exec(command string) error {
	processInfo := exec.Command("/bin/sh", "-c", command)
	_, err := processInfo.Output()
	return err
}

func AddClsactQdiscIntoDev(dev string) error {
	return _exec(
		fmt.Sprintf("tc qdisc add dev %s clsact", dev),
	)
}

func DelClsactQdiscIntoDev(dev string) error {
	return _exec(
		fmt.Sprintf("tc qdisc del dev %s clsact", dev),
	)
}

func AttachIngressBPFIntoDev(dev string, filepath string) error {
	return _exec(
		fmt.Sprintf("tc filter add dev %s ingress bpf direct-action obj %s", dev, filepath),
	)
}

func AttachEgressBPFIntoDev(dev string, filepath string) error {
	return _exec(
		fmt.Sprintf("tc filter add dev %s egress bpf direct-action obj %s", dev, filepath),
	)
}

func ShowBPF(dev string, direct string) (string, error) {
	processInfo := exec.Command(
		"/bin/sh", "-c",
		fmt.Sprintf("tc filter show dev %s %s", dev, direct),
	)
	out, err := processInfo.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
