package utils

import (
	"fmt"
	"sync"
	"testing"
)

func TestUtils(t *testing.T) {
	// test := assert.New(t)

	/***** test random *****/
	wg := sync.WaitGroup{}
	wg.Add(1)
	res := 0
	go func() {
		// TODO: 大概 1000 次调用中会有 3, 5 个左右次会碰巧一样
		// 还是得想办法加锁
		for i := 0; i < 1000; i++ {
			tmp1 := GetRandomNumber(255)
			tmp2 := GetRandomNumber(255)
			if tmp1 == tmp2 {
				res++
			}
		}
		wg.Done()
	}()
	wg.Wait()
	fmt.Println(res)

}
