// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"encoding/json"
	"net"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	stats "github.com/ngaut/gostats"
	topo "github.com/wandoulabs/go-zookeeper/zk"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy/group"
	"github.com/wandoulabs/codis/pkg/proxy/router/topology"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

const MaxSlotNum = models.DEFAULT_SLOT_NUM

type Server struct {
	slots  [MaxSlotNum]*Slot
	evtbus chan interface{}

	conf *Config
	topo *topology.Topology
	info models.ProxyInfo
	pool map[string]*SharedBackendConn

	lastActionSeq int

	net.Listener
	OnSuicide func() error
}

func getEventPath(evt interface{}) string {
	return evt.(topo.Event).Path
}

func needResponse(receivers []string, self models.ProxyInfo) bool {
	var info models.ProxyInfo
	for _, v := range receivers {
		err := json.Unmarshal([]byte(v), &info)
		if err != nil {
			if v == self.Id {
				return true
			}
			return false
		}
		if info.Id == self.Id && info.Pid == self.Pid && info.StartAt == self.StartAt {
			return true
		}
	}
	return false
}

func (s *Server) isValidSlot(i int) bool {
	return i >= 0 && i < MaxSlotNum
}

func (s *Server) getBackendConn(addr string) *SharedBackendConn {
	bc := s.pool[addr]
	if bc != nil {
		bc.IncrRefcnt()
	} else {
		bc = NewSharedBackendConn(addr)
		s.pool[addr] = bc
	}
	return bc
}

func (s *Server) putBackendConn(bc *SharedBackendConn) {
	if bc != nil && bc.Close() {
		delete(s.pool, bc.Addr())
	}
}

func (s *Server) clearSlot(i int) {
	if !s.isValidSlot(i) {
		return
	}
	slot := s.slots[i]
	slot.blockAndWait()

	s.putBackendConn(slot.backend.bc)
	s.putBackendConn(slot.migrate.bc)
	slot.reset()

	slot.unblock()
}

func (s *Server) fillSlot(i int, force bool) {
	if !s.isValidSlot(i) {
		return
	}
	slot := s.slots[i]
	if !force && slot.backend.bc != nil {
		log.Panicf("slot %d already filled, slot: %+v", i, slot)
	}

	slotInfo, slotGroup, err := s.topo.GetSlotByIndex(i)
	if err != nil {
		log.PanicErrorf(err, "get slot by index failed", i)
	}

	var from string
	var addr = group.NewGroup(*slotGroup).Master()
	if slotInfo.State.Status == models.SLOT_STATUS_MIGRATE {
		fromGroup, err := s.topo.GetGroup(slotInfo.State.MigrateStatus.From)
		if err != nil {
			log.PanicErrorf(err, "get migrate from failed")
		}
		from = group.NewGroup(*fromGroup).Master()
		if from == addr {
			log.Panicf("set slot %d migrate from %s to %s", i, from, addr)
		}
	}

	slot.blockAndWait()

	s.putBackendConn(slot.backend.bc)
	s.putBackendConn(slot.migrate.bc)
	slot.reset()

	slot.Info, slot.Group = slotInfo, slotGroup
	if len(addr) != 0 {
		xx := strings.Split(addr, ":")
		if len(xx) >= 1 {
			slot.backend.host = []byte(xx[0])
		}
		if len(xx) >= 2 {
			slot.backend.port = []byte(xx[1])
		}
		slot.backend.addr = addr
		slot.backend.bc = s.getBackendConn(addr)
	}
	if len(from) != 0 {
		slot.migrate.from = from
		slot.migrate.bc = s.getBackendConn(from)
	}

	if slotInfo.State.Status != models.SLOT_STATUS_PRE_MIGRATE {
		slot.unblock()
	}

	if slot.migrate.bc != nil {
		log.Infof("fill slot %d, force %v, backend.addr = %s, migrate.from = %s",
			i, force, slot.backend.addr, slot.migrate.from)
	} else {
		log.Infof("fill slot %d, force %v, backend.addr = %s",
			i, force, slot.backend.addr)
	}
}

func (s *Server) OnSlotRangeChange(param *models.SlotMultiSetParam) {
	log.Warnf("slotRangeChange %+v", param)
	if !s.isValidSlot(param.From) || !s.isValidSlot(param.To) {
		log.Errorf("invalid slot number, %+v", param)
		return
	}
	for i := param.From; i <= param.To; i++ {
		switch param.Status {
		case models.SLOT_STATUS_OFFLINE:
			s.clearSlot(i)
		case models.SLOT_STATUS_ONLINE:
			s.fillSlot(i, true)
		default:
			log.Errorf("can not handle status %v", param.Status)
		}
	}
}

func (s *Server) OnGroupChange(groupId int) {
	log.Warnf("group changed %d", groupId)
	for i, slot := range s.slots {
		if slot.Info.GroupId == groupId {
			s.fillSlot(i, true)
		}
	}
}

type killEvent struct {
}

func (s *Server) registerSignal() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, os.Kill)
	go func() {
		<-c
		log.Info("ctrl-c or SIGTERM found, mark offline server")
		s.evtbus <- &killEvent{}
	}()
}

func (s *Server) responseAction(seq int64) {
	log.Infof("send response seq = %d", seq)
	err := s.topo.DoResponse(int(seq), &s.info)
	if err != nil {
		log.InfoErrorf(err, "send response seq = %d failed", seq)
	}
}

func (s *Server) getActionObject(seq int, target interface{}) {
	act := &models.Action{Target: target}
	err := s.topo.GetActionWithSeqObject(int64(seq), act)
	if err != nil {
		log.PanicErrorf(err, "get action object failed, seq = %d", seq)
	}
	log.Infof("action %+v", act)
}

func (s *Server) checkAndDoTopoChange(seq int) bool {
	act, err := s.topo.GetActionWithSeq(int64(seq))
	if err != nil { //todo: error is not "not exist"
		log.PanicErrorf(err, "action failed, seq = %d", seq)
	}

	if !needResponse(act.Receivers, s.info) { //no need to response
		return false
	}

	log.Warnf("action %v receivers %v", seq, act.Receivers)

	switch act.Type {
	case models.ACTION_TYPE_SLOT_MIGRATE, models.ACTION_TYPE_SLOT_CHANGED,
		models.ACTION_TYPE_SLOT_PREMIGRATE:
		slot := &models.Slot{}
		s.getActionObject(seq, slot)
		s.fillSlot(slot.Id, true)
	case models.ACTION_TYPE_SERVER_GROUP_CHANGED:
		serverGroup := &models.ServerGroup{}
		s.getActionObject(seq, serverGroup)
		s.OnGroupChange(serverGroup.Id)
	case models.ACTION_TYPE_SERVER_GROUP_REMOVE:
	//do not care
	case models.ACTION_TYPE_MULTI_SLOT_CHANGED:
		param := &models.SlotMultiSetParam{}
		s.getActionObject(seq, param)
		s.OnSlotRangeChange(param)
	default:
		log.Panicf("unknown action %+v", act)
	}
	return true
}

func (s *Server) handleMarkOffline() {
	s.topo.Close(s.info.Id)
	if s.OnSuicide == nil {
		s.OnSuicide = func() error {
			log.Panicf("proxy exit: %+v", s.info)
			return nil
		}
	}
	s.OnSuicide()
}

func (s *Server) processAction(e interface{}) {
	if strings.Index(getEventPath(e), models.GetProxyPath(s.topo.ProductName)) == 0 {
		info, err := s.topo.GetProxyInfo(s.info.Id)
		if err != nil {
			log.PanicErrorf(err, "get proxy info failed: %s", s.info.Id)
		}
		if info.State == models.PROXY_STATE_MARK_OFFLINE {
			s.handleMarkOffline()
		}
		return
	}

	//re-watch
	nodes, err := s.topo.WatchChildren(models.GetWatchActionPath(s.topo.ProductName), s.evtbus)
	if err != nil {
		log.PanicErrorf(err, "rewatch children failed")
	}

	seqs, err := models.ExtraSeqList(nodes)
	if err != nil {
		log.PanicErrorf(err, "get seq list failed")
	}

	if len(seqs) == 0 || !s.topo.IsChildrenChangedEvent(e) {
		return
	}

	//get last pos
	index := -1
	for i, seq := range seqs {
		if s.lastActionSeq < seq {
			index = i
			break
		}
	}

	if index < 0 {
		return
	}

	actions := seqs[index:]
	for _, seq := range actions {
		exist, err := s.topo.Exist(path.Join(s.topo.GetActionResponsePath(seq), s.info.Id))
		if err != nil {
			log.PanicErrorf(err, "get action failed")
		}
		if exist {
			continue
		}
		if s.checkAndDoTopoChange(seq) {
			s.responseAction(int64(seq))
		}
	}

	s.lastActionSeq = seqs[len(seqs)-1]
}

func (s *Server) handleTopoEvent() {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()
	for {
		select {
		case e := <-s.evtbus:
			switch e.(type) {
			case *killEvent:
				s.handleMarkOffline()
			default:
				evtPath := getEventPath(e)
				log.Infof("got event %s, %v, lastActionSeq %d", s.info.Id, e, s.lastActionSeq)
				if strings.Index(evtPath, models.GetActionResponsePath(s.conf.productName)) == 0 {
					seq, err := strconv.Atoi(path.Base(evtPath))
					if err != nil {
						log.WarnErrorf(err, "parse action seq failed")
					} else {
						if seq < s.lastActionSeq {
							log.Infof("ignore seq = %d", seq)
							continue
						}
					}
				}
				log.Infof("got event %s, %v, lastActionSeq %d", s.info.Id, e, s.lastActionSeq)
				s.processAction(e)
			}
		case <-ticker.C:
			for _, bc := range s.pool {
				bc.KeepAlive()
			}
		}
	}
}

func (s *Server) waitOnline() {
	for {
		info, err := s.topo.GetProxyInfo(s.info.Id)
		if err != nil {
			log.PanicErrorf(err, "get proxy info failed")
		}
		switch info.State {
		case models.PROXY_STATE_MARK_OFFLINE:
			s.handleMarkOffline()
		case models.PROXY_STATE_ONLINE:
			s.info.State = info.State
			log.Infof("we are online: %s", s.info.Id)
			_, err := s.topo.WatchNode(path.Join(models.GetProxyPath(s.topo.ProductName), s.info.Id), s.evtbus)
			if err != nil {
				log.PanicErrorf(err, "watch node failed")
			}
			return
		}
		select {
		case e := <-s.evtbus:
			switch e.(type) {
			case *killEvent:
				s.handleMarkOffline()
			}
		default: //otherwise ignore it
		}
		log.Warnf("wait to be online: %s", s.info.Id)
		time.Sleep(3 * time.Second)
	}
}

func (s *Server) RegisterAndWait() {
	_, err := s.topo.CreateProxyInfo(&s.info)
	if err != nil {
		log.PanicErrorf(err, "create proxy node failed")
	}
	_, err = s.topo.CreateProxyFenceNode(&s.info)
	if err != nil {
		log.WarnErrorf(err, "create fence node failed")
	}
	s.registerSignal()
	s.waitOnline()
}

func NewServer(addr string, debugVarAddr string, conf *Config) (*Server, error) {
	log.Infof("start proxy with config: %+v", conf)
	s := &Server{
		evtbus:        make(chan interface{}, 1000),
		conf:          conf,
		topo:          topology.NewTopo(conf.productName, conf.zkAddr, conf.fact, conf.provider),
		pool:          make(map[string]*SharedBackendConn),
		lastActionSeq: -1,
	}
	for i := 0; i < MaxSlotNum; i++ {
		s.slots[i] = &Slot{Id: i}
	}

	proxyHost := strings.Split(addr, ":")[0]
	debugHost := strings.Split(debugVarAddr, ":")[0]

	hostname, err := os.Hostname()
	if err != nil {
		log.PanicErrorf(err, "get host name failed")
	}
	if proxyHost == "0.0.0.0" || strings.HasPrefix(proxyHost, "127.0.0.") {
		proxyHost = hostname
	}
	if debugHost == "0.0.0.0" || strings.HasPrefix(debugHost, "127.0.0.") {
		debugHost = hostname
	}

	s.info.Id = conf.proxyId
	s.info.State = models.PROXY_STATE_OFFLINE
	s.info.Addr = proxyHost + ":" + strings.Split(addr, ":")[1]
	s.info.DebugVarAddr = debugHost + ":" + strings.Split(debugVarAddr, ":")[1]
	s.info.Pid = os.Getpid()
	s.info.StartAt = time.Now().String()

	log.Infof("proxy info = %+v", s.info)

	if l, err := net.Listen(conf.proto, addr); err != nil {
		return nil, errors.Trace(err)
	} else {
		s.Listener = l
	}

	stats.Publish("evtbus", stats.StringFunc(func() string {
		return strconv.Itoa(len(s.evtbus))
	}))

	stats.PublishJSONFunc("router", func() string {
		var m = make(map[string]interface{})
		m["ops"] = cmdstats.requests.Get()
		m["cmds"] = getAllOpStats()
		m["info"] = s.info
		m["build"] = map[string]interface{}{
			"version": utils.Version,
			"compile": utils.Compile,
		}
		b, _ := json.Marshal(m)
		return string(b)
	})

	s.RegisterAndWait()

	_, err = s.topo.WatchChildren(models.GetWatchActionPath(conf.productName), s.evtbus)
	if err != nil {
		log.PanicErrorf(err, "watch children failed")
	}

	for i := 0; i < MaxSlotNum; i++ {
		s.fillSlot(i, false)
	}

	go s.handleTopoEvent()

	log.Info("proxy start ok")

	return s, nil
}

func (s *Server) Serve() error {
	for {
		c, err := s.Listener.Accept()
		if err != nil {
			return errors.Trace(err)
		}
		go NewSession(c).Serve(s)
	}
}

func (s *Server) Dispatch(r *Request) error {
	hkey := getHashKey(r.Resp, r.OpStr)
	slot := s.slots[hashSlot(hkey)]
	return slot.forward(r, hkey)
}
