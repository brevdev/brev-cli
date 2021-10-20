package auth

import (
	"math/rand"
	"time"
)

var alphabeticalRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randomAlphabetical(count int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]rune, count)
	for i := range b {
		b[i] = alphabeticalRunes[rand.Intn(len(alphabeticalRunes))]
	}
	return string(b)
}

func randomBytes(count int) []byte {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, count)
	rand.Read(b)
	return b
}
