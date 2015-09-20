package models

const MaxSlotNum = 1024

type Slot struct {
	Id          int    `json:"id"`
	BackendAddr string `json:"backend_addr"`
	MigrateFrom string `json:"migrate_from,omitempty"`
	Locked      bool   `json:"locked,omitempty"`
}

type Group struct {
	Id     int      `json:"id"`
	Master string   `json:"master"`
	Slaves []string `json:"slaves,omitempty"`
}

type SlotMapping struct {
	Id      int `json:"id"`
	GroupId int `json:"group_id"`

	Action *SlotAction `json:"action,omitempty"`
}

const (
	ActionPending   = "pending"
	ActionPreparing = "preparing"
	ActionMigrating = "migrating"
)

type SlotAction struct {
	Index int    `json:"index"`
	State string `json:"state"`

	TargetId int `json:"target_id"`
}
