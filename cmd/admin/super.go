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
	case d["--reinit-product"].(bool):
		t.handleReinitProduct(d)
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

func (t *cmdSuperAdmin) handleReinitProduct(d map[string]interface{}) {
	store := t.newTopomStore(d)
	defer store.Close()

	log.Debugf("reinit product")

	topom := &models.Topom{}
	log.Debugf("acquire lock of product")
	if err := store.Acquire(topom); err != nil {
		log.PanicErrorf(err, "acquire lock of product failed")
	}
	log.Debugf("acquire lock of product OK")

	log.Debugf("list proxy of product")
	plist, err := store.ListProxy()
	if err != nil {
		log.PanicErrorf(err, "list proxy of product failed")
	}
	log.Debugf("list proxy of product OK, total = %d", len(plist))

	for _, p := range plist {
		log.Debugf("remove proxy-[%d]", p.Id)
		if err := store.RemoveProxy(p.Id); err != nil {
			log.PanicErrorf(err, "remove proxy-[%d] failed", p.Id)
		}
		log.Debugf("remove proxy-[%d] OK", p.Id)
	}

	for i := 0; i < models.MaxSlotNum; i++ {
		log.Debugf("reset slot-[%d]", i)
		if slot, err := store.LoadSlotMapping(i); err != nil {
			log.PanicErrorf(err, "load slot-[%d] failed", i)
		} else if slot == nil {
			continue
		}
		if err := store.SaveSlotMapping(i, &models.SlotMapping{Id: i}); err != nil {
			log.PanicErrorf(err, "save slot-[%d] failed", i)
		}
		log.Debugf("reset slot-[%d] OK", i)
	}

	log.Debugf("list group of product")
	glist, err := store.ListGroup()
	if err != nil {
		log.PanicErrorf(err, "list group of product failed")
	}
	log.Debugf("list group of product OK, total = %d", len(glist))

	for _, g := range glist {
		log.Debugf("remove group-[%d]", g.Id)
		if err := store.RemoveGroup(g.Id); err != nil {
			log.PanicErrorf(err, "remove group-[%d] failed", g.Id)
		}
		log.Debugf("remove group-[%d] OK", g.Id)
	}

	log.Debugf("release lock of product")
	if err := store.Release(false); err != nil {
		log.PanicErrorf(err, "release lock of product failed")
	}
	log.Debugf("release lock of product OK")
}
