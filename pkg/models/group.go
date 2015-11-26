// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

import "sort"

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
	Addr string `json:"addr"`

	Action struct {
		Index int    `json:"index,omitempty"`
		State string `json:"state,omitempty"`
	} `json:"action"`
}

func (x *GroupServer) Clone() *GroupServer {
	var dup = *x
	return &dup
}

func (g *Group) Clone() *Group {
	var dup = *g
	dup.Servers = make([]*GroupServer, len(g.Servers))
	for i, x := range g.Servers {
		dup.Servers[i] = x.Clone()
	}
	return &dup
}

func (g *Group) Encode() []byte {
	return jsonEncode(g)
}

func (g *Group) Decode(b []byte) error {
	return jsonDecode(g, b)
}

type groupSorter struct {
	list []*Group
	less func(g1, g2 *Group) bool
}

func (s *groupSorter) Len() int {
	return len(s.list)
}

func (s *groupSorter) Swap(i, j int) {
	s.list[i], s.list[j] = s.list[j], s.list[i]
}

func (s *groupSorter) Less(i, j int) bool {
	return s.less(s.list[i], s.list[j])
}

func SortGroup(list []*Group, less func(g1, g2 *Group) bool) {
	sort.Sort(&groupSorter{list, less})
}

func SortGroupById(gmap map[int]*Group) []*Group {
	list := make([]*Group, 0, len(gmap))
	for _, g := range gmap {
		list = append(list, g)
	}
	SortGroup(list, func(g1, g2 *Group) bool {
		return g1.Id < g2.Id
	})
	return list
}
