package utils

import (
	"fmt"
	"log"
	"os"

	"github.com/sevlyar/go-daemon"
)

func Parent() {
	fmt.Println("这里的 parent")
}

func Child(f func()) {
	f()
}

func StartDeamon(f func()) *os.Process {

	context := new(daemon.Context)
	child, _ := context.Reborn()
	if child != nil {
		Parent()
	} else {
		defer func() {
			if err := context.Release(); err != nil {
				log.Printf("Unable to release pid-file: %s", err.Error())
			}
		}()
		Child(f)
	}
	return child
}
