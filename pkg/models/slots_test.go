// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

import (
	"testing"

	"github.com/CodisLabs/codis/pkg/utils/assert"
	"github.com/wandoulabs/zkhelper"
)

func TestSlots(t *testing.T) {
	fakeZkConn := zkhelper.NewConn()
	path := GetSlotBasePath(productName)
	children, _, _ := fakeZkConn.Children(path)
	assert.Must(len(children) == 0)

	err := InitSlotSet(fakeZkConn, productName, 1024)
	assert.MustNoError(err)

	children, _, _ = fakeZkConn.Children(path)
	assert.Must(len(children) == 1024)

	s, err := GetSlot(fakeZkConn, productName, 1)
	assert.MustNoError(err)

	assert.Must(s.GroupId == -1)

	g := NewServerGroup(productName, 1)
	g.Create(fakeZkConn)

	// test create new group
	_, err = ServerGroups(fakeZkConn, productName)
	assert.MustNoError(err)

	ok, err := g.Exists(fakeZkConn)
	assert.MustNoError(err)
	assert.Must(ok)

	err = SetSlotRange(fakeZkConn, productName, 0, 1023, 1, SLOT_STATUS_ONLINE)
	assert.MustNoError(err)

	s, err = GetSlot(fakeZkConn, productName, 1)
	assert.MustNoError(err)
	assert.Must(s.GroupId == 1)

	err = s.SetMigrateStatus(fakeZkConn, 1, 2)
	assert.MustNoError(err)
	assert.Must(s.GroupId == 2)
	assert.Must(s.State.Status == SLOT_STATUS_MIGRATE)
}
