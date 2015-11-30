package topom

import (
	"testing"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy"
	"github.com/wandoulabs/codis/pkg/utils/assert"
)

var config = NewDefaultConfig()

func init() {
	config.AdminAddr = "0.0.0.0:0"
	config.ProductName = "topom_test"
	config.ProductAuth = "topom_auth"
}

func openTopom() *Topom {
	t, err := New(newMemClient(nil), config)
	assert.MustNoError(err)
	return t
}

func openProxy() (*proxy.Proxy, *proxy.ApiClient, string) {
	config := proxy.NewDefaultConfig()
	config.AdminAddr = "0.0.0.0:0"
	config.ProxyAddr = "0.0.0.0:0"
	config.ProductName = "topom_test"
	config.ProductAuth = "topom_auth"

	s, err := proxy.New(config)
	assert.MustNoError(err)

	c := proxy.NewApiClient(s.Model().AdminAddr)
	c.SetXAuth(config.ProductName, config.ProductAuth, s.Token())
	return s, c, s.Model().AdminAddr
}

func proxyModels(t *Topom) map[string]*models.Proxy {
	stats, err := t.Stats()
	assert.MustNoError(err)
	return stats.Proxy.Models
}

func groupModels(t *Topom) map[int]*models.Group {
	stats, err := t.Stats()
	assert.MustNoError(err)
	return stats.Group.Models
}

func TestTopomClose(x *testing.T) {
	t := openTopom()
	defer t.Close()

	assert.Must(t.IsClosed() == false)
	assert.MustNoError(t.Close())
	assert.Must(t.IsClosed() == true)
}

func TestTopomExclusive(x *testing.T) {
	store := newMemStore()

	t, err := New(newMemClient(store), config)
	assert.MustNoError(err)

	defer t.Close()

	_, err = New(newMemClient(store), config)
	assert.Must(err != nil)

	t.Close()

	_, err = New(newMemClient(store), config)
	assert.MustNoError(err)
}
