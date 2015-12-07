// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

const MaxGroupId = 9999

type Group struct {
	Id      int            `json:"id"`
	Servers []*GroupServer `json:"servers"`

	Promoting struct {
		Index int    `json:"index,omitempty"`
		State string `json:"state,omitempty"`
	} `json:"promoting"`
}

type GroupServer struct {
	Addr string `json:"server"`

	Action struct {
		Index int    `json:"index,omitempty"`
		State string `json:"state,omitempty"`
	} `json:"action"`
}

func (g *Group) Encode() []byte {
	return jsonEncode(g)
}
