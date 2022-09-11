package daemon

import (
	"fmt"
	"log"
	"time"

	"github.com/sevlyar/go-daemon"
)

func Parent() {
	fmt.Println("这里的 parent")
}

func Child() {
	i := 0
	for {
		i++
		time.Sleep(1 * time.Second)
		fmt.Println("这里的 i 是: ", i)
		if i == 10 {
			break
		}
	}
}

func Start() {

	context := new(daemon.Context)
	child, _ := context.Reborn()
	fmt.Println("这里的 child 是: ", child)
	if child != nil {
		Parent()
	} else {
		defer func() {
			if err := context.Release(); err != nil {
				log.Printf("Unable to release pid-file: %s", err.Error())
			}
		}()

		Child()
	}
}
