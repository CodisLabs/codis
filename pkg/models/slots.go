// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

const MaxSlotNum = 1024

type Slot struct {
	Id     int  `json:"id"`
	Locked bool `json:"locked,omitempty"`

	BackendAddr   string     `json:"backend_addr,omitempty"`
	BackendAddrId int        `json:"backend_addr_id,omitempty"`
	MigrateFrom   string     `json:"migrate_from,omitempty"`
	MigrateFromId int        `json:"migrate_from_id,omitempty"`
	ReplicaGroups [][]string `json:"replica_groups,omitempty"`
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
