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

	LoadSlotMapping(slotId int) (*SlotMapping, error)
	SaveSlotMapping(slotId int, slot *SlotMapping) error

	ListProxy() ([]*Proxy, error)
	CreateProxy(proxyId int, proxy *Proxy) error
	RemoveProxy(proxyId int) error

	ListGroup() ([]*Group, error)
	CreateGroup(groupId int, group *Group) error
	UpdateGroup(groupId int, group *Group) error
	RemoveGroup(groupId int) error

	Close() error
}
