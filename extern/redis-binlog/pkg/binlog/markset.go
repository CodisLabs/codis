// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package binlog

import "bytes"

type markSet struct {
	one []byte
	set map[string]bool
}

func (s *markSet) Set(key []byte) {
	if s.one == nil {
		s.one = key
	} else {
		if s.set == nil {
			s.set = make(map[string]bool)
			s.set[string(s.one)] = true
		}
		s.set[string(key)] = true
	}
}

func (s *markSet) Len() int64 {
	if s.set != nil {
		return int64(len(s.set))
	}
	if s.one != nil {
		return 1
	} else {
		return 0
	}
}

func (s *markSet) Has(key []byte) bool {
	if s.set != nil {
		return s.set[string(key)]
	}
	if s.one != nil {
		return bytes.Equal(key, s.one)
	} else {
		return false
	}
}
