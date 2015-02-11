// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package store

import "container/list"

type Batch struct {
	OpList list.List
}

type BatchOpSet struct {
	Key, Value []byte
}

type BatchOpDel struct {
	Key []byte
}

func NewBatch() *Batch {
	return &Batch{}
}

func (bt *Batch) Set(key, value []byte) {
	bt.OpList.PushBack(&BatchOpSet{key, value})
}

func (bt *Batch) Del(key []byte) {
	bt.OpList.PushBack(&BatchOpDel{key})
}

func (bt *Batch) Reset() {
	bt.OpList.Init()
}

func (bt *Batch) Len() int {
	return bt.OpList.Len()
}
