package randstring

import (
	"math/rand"
	"time"
)

const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandString(length uint) string {
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))

	bb := make([]byte, length)

	for idx := range bb {
		bb[idx] = letters[seededRand.Intn(len(letters))]
	}

	return string(bb)
}
