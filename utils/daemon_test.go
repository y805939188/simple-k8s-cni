package utils

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDeamon(t *testing.T) {
	test := assert.New(t)
	child := StartDeamon(func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("111"))
		})
		log.Fatal(http.ListenAndServe(":3190", nil))
	})
	fmt.Println("这里的 child 是: ", child)
	time.Sleep(2 * time.Second)
	test.NotNil(child)
	res, err := http.Get("http://localhost:3190")
	test.Nil(err)
	body, err := ioutil.ReadAll(res.Body)
	test.Nil(err)
	test.Equal(string(body), "111")
	KillByPID(child.Pid)
	_, err = http.Get("http://localhost:3190")
	test.NotNil(err)
}
