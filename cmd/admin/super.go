package main

import (
	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/models/store/etcd"
	"github.com/wandoulabs/codis/pkg/models/store/zk"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

type cmdSuperAdmin struct {
	product struct {
		name string
	}
}

func (t *cmdSuperAdmin) Main(d map[string]interface{}) {
	if s, ok := d["--product-name"].(string); ok {
		t.product.name = s
	}

	log.Debugf("args.product.name = %s", t.product.name)

	switch {
	case d["--remove-lock"].(bool):
		t.handleRemoveLock(d)
	}
}

func (t *cmdSuperAdmin) newTopomStore(d map[string]interface{}) models.Store {
	if !utils.IsValidName(t.product.name) {
		log.Panicf("invalid product name")
	}

	switch {
	case d["--zookeeper"] != nil:
		s, err := zkstore.NewStore(d["--zookeeper"].(string), t.product.name)
		if err != nil {
			log.PanicErrorf(err, "create zkstore failed")
		}
		return s
	case d["--etcd"] != nil:
		s, err := etcdstore.NewStore(d["--etcd"].(string), t.product.name)
		if err != nil {
			log.PanicErrorf(err, "create etcdstore failed")
		}
		return s
	}

	log.Panicf("nil store for topom")
	return nil
}

func (t *cmdSuperAdmin) handleRemoveLock(d map[string]interface{}) {
	store := t.newTopomStore(d)
	defer store.Close()

	log.Debugf("force remove-lock")
	if err := store.Release(true); err != nil {
		log.PanicErrorf(err, "force remove-lock failed")
	}
	log.Debugf("force remove-lock OK")
}
