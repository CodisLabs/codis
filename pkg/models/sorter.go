// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

import "sort"

type GroupSlice []*Group

func (s GroupSlice) Len() int {
	return len(s)
}

func (s GroupSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s GroupSlice) Less(i, j int) bool {
	return s[i].Id < s[j].Id
}

func SortGroup(group map[int]*Group) []*Group {
	slice := make([]*Group, 0, len(group))
	for _, g := range group {
		slice = append(slice, g)
	}
	sort.Sort(GroupSlice(slice))
	return slice
}

type ProxySlice []*Proxy

func (s ProxySlice) Len() int {
	return len(s)
}

func (s ProxySlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s ProxySlice) Less(i, j int) bool {
	return s[i].Id < s[j].Id
}

func SortProxy(proxy map[string]*Proxy) []*Proxy {
	slice := make([]*Proxy, 0, len(proxy))
	for _, p := range proxy {
		slice = append(slice, p)
	}
	sort.Sort(ProxySlice(slice))
	return slice
}
