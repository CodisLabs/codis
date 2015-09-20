package rpc

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"os"
	"time"
)

func NewToken() string {
	hostname, _ := os.Hostname()
	c := make([]byte, 16)
	rand.Read(c)

	s := fmt.Sprintf("%s-%d-%x", hostname, time.Now().UnixNano(), c)
	b := md5.Sum([]byte(s))
	return fmt.Sprintf("%x", b)
}

func NewXAuth(auth, token string) string {
	s := fmt.Sprintf("%s-%s", auth, token)
	b := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", b[:16])
}
