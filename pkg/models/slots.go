// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

import "sort"

const MaxSlotNum = 1024

type Slot struct {
	Id          int    `json:"id"`
	BackendAddr string `json:"backend_addr"`
	MigrateFrom string `json:"migrate_from,omitempty"`
	Locked      bool   `json:"locked,omitempty"`
}

type SlotMapping struct {
	Id      int `json:"id"`
	GroupId int `json:"group_id"`

	Action struct {
		Index    int    `json:"index,omitempty"`
		State    string `json:"state,omitempty"`
		TargetId int    `json:"target_id,omitempty"`
	} `json:"action"`
}

const (
	ActionNothing   = ""
	ActionPending   = "pending"
	ActionPreparing = "preparing"
	ActionMigrating = "migrating"
)

func (s *SlotMapping) Encode() []byte {
	return jsonEncode(s)
}

func (s *SlotMapping) Decode(b []byte) error {
	return jsonDecode(s, b)
}

type slotsSorter struct {
	list []*SlotMapping
	less func(s1, s2 *SlotMapping) bool
}

func (s *slotsSorter) Len() int {
	return len(s.list)
}

func (s *slotsSorter) Swap(i, j int) {
	s.list[i], s.list[j] = s.list[j], s.list[i]
}

func (s *slotsSorter) Less(i, j int) bool {
	return s.less(s.list[i], s.list[j])
}

func SortSlots(list []*SlotMapping, less func(s1, s2 *SlotMapping) bool) {
	sort.Sort(&slotsSorter{list, less})
}
