// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package rpc

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"net"
	"sort"
)

func NewToken(segs ...string) string {
	var list []string
	ifs, _ := net.Interfaces()
	for _, i := range ifs {
		addr := i.HardwareAddr.String()
		if addr != "" {
			list = append(list, addr)
		}
	}
	sort.Strings(list)

	t := &bytes.Buffer{}
	fmt.Fprintf(t, "Codis-Token@%v", list)
	for _, s := range segs {
		fmt.Fprintf(t, "-{%s}", s)
	}
	b := md5.Sum(t.Bytes())
	return fmt.Sprintf("%x", b)
}

func NewXAuth(segs ...string) string {
	t := &bytes.Buffer{}
	fmt.Fprintf(t, "Codis-XAuth")
	for _, s := range segs {
		fmt.Fprintf(t, "-[%s]", s)
	}
	b := sha256.Sum256(t.Bytes())
	return fmt.Sprintf("%x", b[:16])
}
