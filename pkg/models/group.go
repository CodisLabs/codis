// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

import "sort"

type Group struct {
	Id        int      `json:"id"`
	Servers   []string `json:"servers"`
	Promoting bool     `json:"promoting,omitempty"`
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
