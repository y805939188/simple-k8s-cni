package client

import (
	"os"
	"testcni/helper"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClient(t *testing.T) {
	test := assert.New(t)
	paths, err := helper.GetHostAuthenticationInfoPath()
	test.Nil(err)
	Init(paths.CaPath, paths.CertPath, paths.KeyPath)
	client, err := GetLightK8sClient()
	test.Nil(err)
	nodes, err := client.Get().Nodes()
	test.Nil(err)
	test.NotNil(nodes)
	test.Len(nodes.Items, 3)
	hostname, err := os.Hostname()
	test.Nil(err)
	node, err := client.Get().Node(hostname)
	test.Nil(err)
	test.NotNil(node)
	test.Equal(node.ObjectMeta.Name, hostname)
}
