package bpf_map

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWatcher(t *testing.T) {
	test := assert.New(t)
	mm, err := GetMapsManager()
	test.Nil(err)
	test.NotNil(mm)

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
			IfIndex: 2,
			LxcID:   3,
			MAC:     4,
			NodeMAC: 5,
		},
	)
	test.Nil(err)

	err = mm.SetPodMap(
		PodNodeMapKey{IP: 10},
		PodNodeMapValue{IP: 11},
	)
	test.Nil(err)

	err = mm.SetNodeLocalMap(
		LocalNodeMapKey{IP: 666},
		LocalNodeMapValue{IfIndex: 777},
	)
	test.Nil(err)

	/************ test get ************/
	lxc, err := mm.GetLxcMapValue(EndpointMapKey{IP: 1})
	test.Nil(err)
	test.EqualValues(lxc, &EndpointMapInfo{
		IfIndex: 2,
		LxcID:   3,
		MAC:     4,
		NodeMAC: 5,
	})

	pod, err := mm.GetPodMapValue(PodNodeMapKey{IP: 10})
	test.Nil(err)
	test.EqualValues(pod, &PodNodeMapValue{IP: 11})

	local, err := mm.GetNodeLocalMapValue(LocalNodeMapKey{IP: 666})
	test.Nil(err)
	test.EqualValues(local, &LocalNodeMapValue{IfIndex: 777})
}
