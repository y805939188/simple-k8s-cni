package tc

import (
	"fmt"
	"testcni/nettools"
	"testing"

	"github.com/stretchr/testify/assert"
)

// clang -O2 -emit-llvm -c tc_test.c -o - | llc -march=bpf -filetype=obj -o tc_test.o
// 编译 tc_test.c
func TestTC(t *testing.T) {
	test := assert.New(t)
	_, err := nettools.CreateVxlanAndUp("ding_test", 1500)
	test.Nil(err)
	err = AddClsactQdiscIntoDev("ding_test")
	test.Nil(err)
	err = AttachIngressBPFIntoDev("ding_test", "./tc_test.o")
	test.Nil(err)
	err = AttachEgressBPFIntoDev("ding_test", "./tc_test.o")
	test.Nil(err)
	out, err := ShowBPF("ding_test", "ingress")
	test.Nil(err)
	test.Contains(out, "classifier")
	fmt.Println("ingress 方向: ", out)
	out, err = ShowBPF("ding_test", "egress")
	test.Nil(err)
	test.Contains(out, "classifier")
	fmt.Println("egress 方向", out)
	err = DelClsactQdiscIntoDev("ding_test")
	test.Nil(err)
}
