// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package rpc

import (
	"bytes"
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

func NewXAuth(segs ...string) string {
	t := &bytes.Buffer{}
	t.WriteString("Codis-XAuth")
	for _, s := range segs {
		t.WriteString("-[")
		t.WriteString(s)
		t.WriteString("]")
	}
	b := sha256.Sum256(t.Bytes())
	return fmt.Sprintf("%x", b[:16])
}
