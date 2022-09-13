package tc

import (
	"testcni/nettools"
	"testing"

	"github.com/stretchr/testify/assert"
)

// 编译 tc_test.c: clang -O2 -emit-llvm -c tc_test.c -o - | llc -march=bpf -filetype=obj -o tc_test.o
func TestTC(t *testing.T) {
	test := assert.New(t)
	_, err := nettools.CreateVxlanAndUp("ding_test", 1500)
	test.Nil(err)

	/********* test attach clsact *********/
	exist := ExistClsact("ding_test")
	test.False(exist)
	err = AddClsactQdiscIntoDev("ding_test")
	test.Nil(err)
	exist = ExistClsact("ding_test")
	test.True(exist)

	/********* test attach ingress *********/
	exist = ExistIngress("ding_test")
	test.False(exist)
	err = AttachIngressBPFIntoDev("ding_test", "./tc_test.o")
	test.Nil(err)
	exist = ExistIngress("ding_test")
	test.True(exist)

	/********* test attach egress *********/
	exist = ExistEgress("ding_test")
	test.False(exist)
	err = AttachEgressBPFIntoDev("ding_test", "./tc_test.o")
	test.Nil(err)
	exist = ExistEgress("ding_test")
	test.True(exist)

	/********* test show *********/
	out, err := ShowBPF("ding_test", "ingress")
	test.Nil(err)
	test.Contains(out, "classifier")
	// fmt.Println("ingress 方向: ", out)
	out, err = ShowBPF("ding_test", "egress")
	test.Nil(err)
	test.Contains(out, "classifier")
	// fmt.Println("egress 方向", out)

	/********* test del clsact *********/
	err = DelClsactQdiscIntoDev("ding_test")
	test.Nil(err)

	exist = ExistIngress("ding_test")
	test.False(exist)
	exist = ExistEgress("ding_test")
	test.False(exist)
	exist = ExistClsact("ding_test")
	test.False(exist)
}
