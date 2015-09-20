package models

type Topom struct {
	Token     string `json:"token"`
	StartTime string `json:"start_time"`
	AdminAddr string `json:"admin_addr"`

	Pid int    `json:"pid"`
	Pwd string `json:"pwd"`
}

type Store interface {
	Acquire(topom *Topom) error
	Release(topom *Topom) error

	GetMapping(i int) (*SlotMapping, error)
	UpdateMapping(slot *SlotMapping) error

	ListProxy() ([]*Proxy, error)
	CreateProxy(proxy *Proxy) error
	RemoveProxy(id int) error

	ListGroup() ([]*Group, error)
	CreateGroup(group *Group) error
	UpdateGroup(group *Group) error
	RemoveGroup(id int) error

	Close() error
}
