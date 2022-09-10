package helper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientHelper(t *testing.T) {
	test := assert.New(t)
	paths, err := GetHostAuthenticationInfoPath()
	test.Nil(err)
	test.EqualValues(paths, &AuthenticationInfoPath{
		CaPath:   "/opt/testcni/ca.crt",
		CertPath: "/opt/testcni/cert.crt",
		KeyPath:  "/opt/testcni/key.key",
	})
}
