// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

import "strings"

const (
	ForwardSync = iota
	ForwardSemiAsync
)

const MaxSlotNum = 1024

type Slot struct {
	Id     int  `json:"id"`
	Locked bool `json:"locked,omitempty"`

	BackendAddr        string `json:"backend_addr,omitempty"`
	BackendAddrGroupId int    `json:"backend_addr_group_id,omitempty"`
	MigrateFrom        string `json:"migrate_from,omitempty"`
	MigrateFromGroupId int    `json:"migrate_from_group_id,omitempty"`

	ForwardMethod int `json:"forward_method,omitempty"`

	ReplicaGroups [][]string `json:"replica_groups,omitempty"`
}

func ParseForwardMethod(s string) (int, bool) {
	switch strings.ToUpper(s) {
	default:
		return ForwardSync, false
	case "SYNC":
		return ForwardSync, true
	case "SEMI-ASYNC":
		return ForwardSemiAsync, true
	}
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

func (m *SlotMapping) Encode() []byte {
	return jsonEncode(m)
}
