package models

import (
	"encoding/json"

	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

type Store interface {
	Acquire(name string, topom *Topom) error
	Release(force bool) error

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

type Topom struct {
	StartTime string `json:"start_time"`
	AdminAddr string `json:"admin_addr"`

	ProductName string `json:"product_name"`

	Pid int    `json:"pid"`
	Pwd string `json:"pwd"`
}

func (t *Topom) Encode() []byte {
	return jsonEncode(t)
}

func jsonEncode(v interface{}) []byte {
	b, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		log.PanicErrorf(err, "encode to json failed")
	}
	return b
}

func jsonDecode(v interface{}, b []byte) error {
	if err := json.Unmarshal(b, v); err != nil {
		return errors.Trace(err)
	}
	return nil
}
