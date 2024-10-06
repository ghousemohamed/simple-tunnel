package cmd

import (
	"math/rand"
	"time"
)

func GenerateRandomSubdomain(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	rand.New(rand.NewSource(time.Now().UnixNano()))
	subdomain := make([]byte, length)
	for i := range subdomain {
		subdomain[i] = charset[rand.Intn(len(charset))]
	}
	return string(subdomain)
}
