package topom

import (
	"testing"

	"github.com/wandoulabs/codis/pkg/utils/assert"
)

func TestCreateProxy(x *testing.T) {
	t := openTopom()
	defer t.Close()

	_, c1, addr1 := openProxy()
	defer c1.Shutdown()

	assert.Must(len(proxyModels(t)) == 0)
	assert.MustNoError(t.CreateProxy(addr1))
	assert.Must(len(proxyModels(t)) == 1)
	assert.Must(t.CreateProxy(addr1) != nil)
	assert.Must(len(proxyModels(t)) == 1)

	_, c2, addr2 := openProxy()
	defer c2.Shutdown()

	assert.MustNoError(c2.Shutdown())
	assert.Must(len(proxyModels(t)) == 1)
	assert.Must(t.CreateProxy(addr2) != nil)
	assert.Must(len(proxyModels(t)) == 1)

	assert.Must(len(proxyModels(t)) == 1)
	assert.MustNoError(c1.Shutdown())
	assert.Must(len(proxyModels(t)) == 1)
}

func TestRemoveProxy(x *testing.T) {
	t := openTopom()
	defer t.Close()

	p1, c1, addr1 := openProxy()
	defer c1.Shutdown()

	assert.Must(len(proxyModels(t)) == 0)
	assert.MustNoError(t.CreateProxy(addr1))
	assert.Must(len(proxyModels(t)) == 1)
	assert.MustNoError(t.RemoveProxy(p1.Token(), false))
	assert.Must(len(proxyModels(t)) == 0)

	p2, c2, addr2 := openProxy()
	defer c2.Shutdown()

	assert.Must(len(proxyModels(t)) == 0)
	assert.MustNoError(t.CreateProxy(addr2))
	assert.Must(len(proxyModels(t)) == 1)
	assert.MustNoError(c2.Shutdown())
	assert.Must(t.RemoveProxy(p2.Token(), false) != nil)
	assert.MustNoError(t.RemoveProxy(p2.Token(), true))
	assert.Must(len(proxyModels(t)) == 0)
}
