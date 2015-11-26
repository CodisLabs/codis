// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

import "sort"

type Proxy struct {
	Id        int    `json:"id,omitempty"`
	Token     string `json:"token"`
	StartTime string `json:"start_time"`
	AdminAddr string `json:"admin_addr"`

	ProtoType string `json:"proto_type"`
	ProxyAddr string `json:"proxy_addr"`

	ProductName string `json:"product_name"`

	Pid int    `json:"pid"`
	Pwd string `json:"pwd"`
	Sys string `json:"sys"`
}

func (p *Proxy) Clone() *Proxy {
	var dup = *p
	return &dup
}

func (p *Proxy) Encode() []byte {
	return jsonEncode(p)
}

func (p *Proxy) Decode(b []byte) error {
	return jsonDecode(p, b)
}

type proxySorter struct {
	list []*Proxy
	less func(p1, p2 *Proxy) bool
}

func (s *proxySorter) Len() int {
	return len(s.list)
}

func (s *proxySorter) Swap(i, j int) {
	s.list[i], s.list[j] = s.list[j], s.list[i]
}

func (s *proxySorter) Less(i, j int) bool {
	return s.less(s.list[i], s.list[j])
}

func SortProxy(list []*Proxy, less func(p1, p2 *Proxy) bool) {
	sort.Sort(&proxySorter{list, less})
}

func SortProxyById(pmap map[string]*Proxy) []*Proxy {
	list := make([]*Proxy, 0, len(pmap))
	for _, p := range pmap {
		list = append(list, p)
	}
	SortProxy(list, func(p1, p2 *Proxy) bool {
		return p1.Id < p2.Id
	})
	return list
}
