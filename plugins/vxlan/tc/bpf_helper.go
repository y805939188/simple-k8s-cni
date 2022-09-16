package tc

import (
	"errors"
	"testcni/consts"
)

type BPF_TC_DIRECT string

const (
	INGRESS BPF_TC_DIRECT = "ingress"
	EGRESS  BPF_TC_DIRECT = "egress"
)

func GetVethIngressPath() string {
	return consts.KUBE_TEST_CNI_DEFAULT_PATH + "/veth_ingress.o"
}

func GetVxlanIngressPath() string {
	return consts.KUBE_TEST_CNI_DEFAULT_PATH + "/vxlan_ingress.o"
}

func GetVxlanEgressPath() string {
	return consts.KUBE_TEST_CNI_DEFAULT_PATH + "/vxlan_egress.o"
}

func TryAttachBPF(dev string, direct BPF_TC_DIRECT, program string) error {
	// 如果还没有 clsact 这根儿管子就先尝试 add 一个
	if !ExistClsact(dev) {
		err := AddClsactQdiscIntoDev(dev)
		if err != nil {
			return err
		}
	}

	// 如果当前 dev 上已经有 ingress 或者 egress 就跳过
	switch direct {
	case INGRESS:
		if ExistIngress(dev) {
			return nil
		}
		return AttachIngressBPFIntoDev(dev, program)
	case EGRESS:
		if ExistEgress(dev) {
			return nil
		}
		return AttachEgressBPFIntoDev(dev, program)
	}
	return errors.New("unknow error occurred in TryAttachBPF")
}

func DetachBPF(dev string) error {
	return DelClsactQdiscIntoDev(dev)
}
