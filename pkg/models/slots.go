package models

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
		Index    int    `json:"index"`
		State    string `json:"state"`
		TargetId int    `json:"target_id"`
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
