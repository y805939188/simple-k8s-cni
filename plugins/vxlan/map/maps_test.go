package bpf_map

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMap(t *testing.T) {
	test := assert.New(t)
	mm, err := GetMapsManager()
	test.Nil(err)
	test.NotNil(mm)

	nums, err := mm.DeleteAllPodMap()
	test.Nil(err)
	fmt.Printf("删除掉了 %d 个 keys", nums)
	// ip := utils.InetIpToUInt32("10.244.134.23")
	// v, err := mm.GetLxcMapValue(EndpointMapKey{IP: ip})
	// test.Nil(err)
	// fmt.Println(v)
	// return

	/************ test create ************/
	lxcMap, err := mm.CreateLxcMap()
	test.Nil(err)
	test.NotNil(lxcMap)

	podMap, err := mm.CreatePodMap()
	test.Nil(err)
	test.NotNil(podMap)

	localMap, err := mm.CreateNodeLocalMap()
	test.Nil(err)
	test.NotNil(localMap)

	/************ test set ************/
	err = mm.SetLxcMap(
		EndpointMapKey{IP: 1},
		EndpointMapInfo{
			IfIndex:    2,
			LxcIfIndex: 3,
			MAC:        [8]byte{4},
			NodeMAC:    [8]byte{5},
		},
	)
	test.Nil(err)

	err = mm.SetPodMap(
		PodNodeMapKey{IP: 10},
		PodNodeMapValue{IP: 11},
	)
	test.Nil(err)

	err = mm.SetNodeLocalMap(
		LocalNodeMapKey{Type: 666},
		LocalNodeMapValue{IfIndex: 777},
	)
	test.Nil(err)

	nums, err = mm.BatchSetLxcMap(
		[]EndpointMapKey{
			{IP: 3},
			{IP: 4},
		},
		[]EndpointMapInfo{
			{
				IfIndex:    5,
				LxcIfIndex: 5,
				MAC:        [8]byte{5},
				NodeMAC:    [8]byte{5},
			},
			{
				IfIndex:    6,
				LxcIfIndex: 6,
				MAC:        [8]byte{6},
				NodeMAC:    [8]byte{6},
			},
		},
	)
	test.Nil(err)
	test.Equal(nums, 2)

	nums, err = mm.BatchSetPodMap(
		[]PodNodeMapKey{
			{IP: 20},
			{IP: 30},
		},
		[]PodNodeMapValue{
			{IP: 21},
			{IP: 31},
		},
	)
	test.Nil(err)
	test.Equal(nums, 2)

	nums, err = mm.BatchSetNodeLocalMap(
		[]LocalNodeMapKey{
			{Type: 100},
			{Type: 200},
		},
		[]LocalNodeMapValue{
			{IfIndex: 101},
			{IfIndex: 201},
		},
	)
	test.Nil(err)
	test.Equal(nums, 2)

	/************ test get ************/
	lxc, err := mm.GetLxcMapValue(EndpointMapKey{IP: 1})
	test.Nil(err)
	test.EqualValues(lxc, &EndpointMapInfo{
		IfIndex:    2,
		LxcIfIndex: 3,
		MAC:        [8]byte{4},
		NodeMAC:    [8]byte{5},
	})

	pod, err := mm.GetPodMapValue(PodNodeMapKey{IP: 10})
	test.Nil(err)
	test.EqualValues(pod, &PodNodeMapValue{IP: 11})

	local, err := mm.GetNodeLocalMapValue(LocalNodeMapKey{Type: 666})
	test.Nil(err)
	test.EqualValues(local, &LocalNodeMapValue{IfIndex: 777})

	/************ test del ************/
	err = mm.DelLxcMap(EndpointMapKey{IP: 1})
	test.Nil(err)

	err = mm.DelPodMap(PodNodeMapKey{IP: 10})
	test.Nil(err)

	err = mm.DelNodeLocalMap(LocalNodeMapKey{Type: 666})
	test.Nil(err)

	nums, err = mm.BatchDelLxcMap(
		[]EndpointMapKey{
			{IP: 3},
			{IP: 4},
		},
	)
	test.Nil(err)
	test.Equal(nums, 2)

	nums, err = mm.BatchDelPodMap(
		[]PodNodeMapKey{
			{IP: 20},
			{IP: 30},
		},
	)
	test.Nil(err)
	test.Equal(nums, 2)

	nums, err = mm.BatchDelNodeLocalMap(
		[]LocalNodeMapKey{
			{Type: 100},
			{Type: 200},
		},
	)
	test.Nil(err)
	test.Equal(nums, 2)
}
