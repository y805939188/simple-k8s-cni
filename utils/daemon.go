package utils

import (
	"os"

	"github.com/sevlyar/go-daemon"
)

func Parent() {}

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
				WriteLog("Unable to release pid-file: %s", err.Error())
			}
		}()
		Child(f)
	}
	return child
}
