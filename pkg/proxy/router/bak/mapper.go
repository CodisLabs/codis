// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"bytes"
	"hash/crc32"

	"github.com/wandoulabs/codis/pkg/models"
)

const (
	HASHTAG_START = '{'
	HASHTAG_END   = '}'
)

func mapKey2Slot(key []byte) int {
	hashKey := key
	//hash tag support
	htagStart := bytes.IndexByte(key, HASHTAG_START)
	if htagStart >= 0 {
		htagEnd := bytes.IndexByte(key[htagStart:], HASHTAG_END)
		if htagEnd >= 0 {
			hashKey = key[htagStart+1 : htagStart+htagEnd]
		}
	}

	return int(crc32.ChecksumIEEE(hashKey) % models.DEFAULT_SLOT_NUM)
}
