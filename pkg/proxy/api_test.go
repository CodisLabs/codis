package proxy_test

import (
	"net"
	"testing"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/assert"
)

var config = proxy.NewDefaultConfig()

func openProxy() (*proxy.Proxy, string) {
	l, err := net.Listen("tcp", "0.0.0.0:0")
	assert.MustNoError(err)

	config.ProxyAddr = "0.0.0.0:0"
	config.AdminAddr = l.Addr().String()

	l.Close()

	s, err := proxy.New(config)
	assert.MustNoError(err)

	return s, s.GetConfig().AdminAddr
}

func TestInfo(x *testing.T) {
	s, addr := openProxy()
	defer s.Close()

	var c = proxy.NewApiClient(addr)

	sum, err := c.Summary()
	assert.MustNoError(err)
	assert.Must(sum.Version == utils.Version)
	assert.Must(sum.Compile == utils.Compile)
	assert.Must(sum.Model.Token == s.GetToken())
}

func TestStats(x *testing.T) {
	s, addr := openProxy()
	defer s.Close()

	var c = proxy.NewApiClient(addr)

	c.SetXAuth(config.ProductName, config.ProductAuth, "")
	_, err1 := c.Stats()
	assert.Must(err1 != nil)

	c.SetXAuth(config.ProductName, config.ProductAuth, s.GetToken())
	_, err2 := c.Stats()
	assert.MustNoError(err2)
}

func verifySlots(c *proxy.ApiClient, expect map[int]*models.Slot) {
	sum, err := c.Summary()
	assert.MustNoError(err)

	slots := sum.Slots
	assert.Must(len(slots) == models.MaxSlotNum)

	for i, slot := range expect {
		if slot != nil {
			assert.Must(slots[i].Id == i)
			assert.Must(slot.Locked == slots[i].Locked)
			assert.Must(slot.BackendAddr == slots[i].BackendAddr)
			assert.Must(slot.MigrateFrom == slots[i].MigrateFrom)
		}
	}
}

func TestFillSlot(x *testing.T) {
	s, addr := openProxy()
	defer s.Close()

	var c = proxy.NewApiClient(addr)
	c.SetXAuth(config.ProductName, config.ProductAuth, s.GetToken())

	expect := make(map[int]*models.Slot)

	for i := 0; i < 16; i++ {
		slot := &models.Slot{
			Id:          i,
			Locked:      i%2 == 0,
			BackendAddr: "x.x.x.x:xxxx",
		}
		assert.MustNoError(c.FillSlot(slot))
		expect[i] = slot
	}
	verifySlots(c, expect)

	slots := []*models.Slot{}
	for i := 0; i < 16; i++ {
		slot := &models.Slot{
			Id:          i,
			Locked:      i%2 != 0,
			BackendAddr: "y.y.y.y:yyyy",
			MigrateFrom: "x.x.x.x:xxxx",
		}
		slots = append(slots, slot)
		expect[i] = slot
	}
	assert.MustNoError(c.FillSlot(slots...))
	verifySlots(c, expect)
}

func TestStartAndShutdown(x *testing.T) {
	s, addr := openProxy()
	defer s.Close()

	var c = proxy.NewApiClient(addr)
	c.SetXAuth(config.ProductName, config.ProductAuth, s.GetToken())

	expect := make(map[int]*models.Slot)

	for i := 0; i < 16; i++ {
		slot := &models.Slot{
			Id:          i,
			BackendAddr: "x.x.x.x:xxxx",
		}
		assert.MustNoError(c.FillSlot(slot))
		expect[i] = slot
	}
	verifySlots(c, expect)

	err1 := c.Start()
	assert.MustNoError(err1)

	err2 := c.Shutdown()
	assert.MustNoError(err2)

	err3 := c.Start()
	assert.Must(err3 != nil)
}
