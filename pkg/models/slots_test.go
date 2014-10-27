package models

import (
	"codis/pkg/zkhelper"
	"testing"
)

func TestSlots(t *testing.T) {
	fakeZkConn := zkhelper.NewFakeConn()
	path := GetSlotBasePath(productName)
	children, _, _ := fakeZkConn.Children(path)
	if len(children) != 0 {
		t.Error("slot is no empty")
	}

	err := InitSlotSet(fakeZkConn, productName, 1024)
	if err != nil {
		t.Error(err)
	}

	children, _, _ = fakeZkConn.Children(path)
	if len(children) != 1024 {
		t.Error("init slots error")
	}

	s, err := GetSlot(fakeZkConn, productName, 1)
	if err != nil {
		t.Error(err)
	}

	if s.GroupId != -1 {
		t.Error("init slots error")
	}

	err = SetSlotRange(fakeZkConn, productName, 0, 1023, 1, SLOT_STATUS_ONLINE)
	if err != nil {
		t.Error(err)
	}

	s, err = GetSlot(fakeZkConn, productName, 1)
	if err != nil {
		t.Error(err)
	}

	if s.GroupId != 1 {
		t.Error("range set error")
	}

	err = s.Migrate(fakeZkConn, 1, 2)
	if err != nil {
		t.Error(err)
	}

	if s.GroupId != 2 || s.State.Status != SLOT_STATUS_MIGRATE {
		t.Error("migrate error")
	}

}
