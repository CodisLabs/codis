package models

type Topom struct {
	StartTime string `json:"start_time"`
	AdminAddr string `json:"admin_addr"`

	Pid int    `json:"pid"`
	Pwd string `json:"pwd"`
}

type Store interface {
	Acquire(topom *Topom) error
	Release() error

	LoadSlotMapping(i int) (*SlotMapping, error)
	SaveSlotMapping(slot *SlotMapping) error

	ListProxy() ([]*Proxy, error)
	CreateProxy(proxy *Proxy) error
	RemoveProxy(id int) error

	ListGroup() ([]*Group, error)
	CreateGroup(group *Group) error
	UpdateGroup(group *Group) error
	RemoveGroup(id int) error

	Close() error
}
