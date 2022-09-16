package utils

import (
	"math/rand"
	"time"
)

func GetRandomNumber(max int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max)
}
