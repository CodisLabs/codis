package topom

import (
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/utils"
)

type migrationTask struct {
	s *Topom

	slotId int
}

func (s *Topom) newMigrationTask() *migrationTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil
	}

	var x *models.SlotMapping
	for _, m := range s.mappings {
		if m.Action.State != models.ActionNothing {
			if x == nil || x.Action.Index > m.Action.Index {
				x = m
			}
		}
	}
	if x != nil {
		return &migrationTask{s, x.Id}
	}
	return nil
}

func (s *Topom) noopInterval() {
	var ms int
	for {
		if d := int(s.intvl.Get()) - ms; d <= 0 {
			return
		} else {
			d = utils.MinInt(d, 50)
			time.Sleep(time.Millisecond * time.Duration(d))
			select {
			case <-s.exit.C:
				return
			default:
				ms += d
			}
		}
	}
}

func (s *Topom) daemonMigration() {
	for !s.IsClosed() {
		var t = s.newMigrationTask()
		if t == nil {
			time.Sleep(time.Millisecond * 250)
		} else if t.run() != nil {
			time.Sleep(time.Second * 3)
		}
	}
}

func (t *migrationTask) run() error {
	if err := t.s.migrationPrepare(t.slotId); err != nil {
		return err
	}
	for {
		n, err := t.s.migrationProcess(t.slotId)
		if err != nil {
			return err
		}
		if n == 0 {
			return t.s.migrationComplete(t.slotId)
		}
		t.s.noopInterval()
	}
}
