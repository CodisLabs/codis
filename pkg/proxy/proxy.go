// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"encoding/json"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	topo "github.com/wandoulabs/go-zookeeper/zk"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy/router"
	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

type Server struct {
	conf   *Config
	topo   *Topology
	info   models.ProxyInfo
	groups map[int]int

	lastActionSeq int

	evtbus   chan interface{}
	router   *router.Router
	listener net.Listener

	kill chan interface{}
	wait sync.WaitGroup
	stop sync.Once
}

func New(addr string, debugVarAddr string, conf *Config) *Server {
	log.Infof("create proxy with config: %+v", conf)

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

	s := &Server{conf: conf, lastActionSeq: -1, groups: make(map[int]int)}
	s.topo = NewTopo(conf.productName, conf.zkAddr, conf.fact, conf.provider, conf.zkSessionTimeout)
	s.info.Id = conf.proxyId
	s.info.State = models.PROXY_STATE_OFFLINE
	s.info.Addr = proxyHost + ":" + strings.Split(addr, ":")[1]
	s.info.DebugVarAddr = debugHost + ":" + strings.Split(debugVarAddr, ":")[1]
	s.info.Pid = os.Getpid()
	s.info.StartAt = time.Now().String()
	s.kill = make(chan interface{})

	log.Infof("proxy info = %+v", s.info)

	if l, err := net.Listen(conf.proto, addr); err != nil {
		log.PanicErrorf(err, "open listener failed")
	} else {
		s.listener = l
	}
	s.router = router.NewWithAuth(conf.passwd)
	s.evtbus = make(chan interface{}, 1024)

	s.register()

	s.wait.Add(1)
	go func() {
		defer func() {
			s.router.Close()
			s.listener.Close()
			s.wait.Done()
		}()
		if s.info.State != models.PROXY_STATE_ONLINE {
			return
		}
		_, err := s.topo.WatchChildren(models.GetWatchActionPath(s.conf.productName), s.evtbus)
		if err != nil {
			log.PanicErrorf(err, "watch children failed")
		}
		for i := 0; i < router.MaxSlotNum; i++ {
			s.fillSlot(i)
		}
		log.Infof("proxy is ready for serving")

		s.handleTopoEvent()
	}()

	return s
}

func (s *Server) Info() models.ProxyInfo {
	return s.info
}

func (s *Server) Close() error {
	s.stop.Do(func() {
		close(s.kill)
	})
	s.wait.Wait()
	return nil
}

func (s *Server) Join() {
	s.wait.Wait()
}

func (s *Server) register() {
	if _, err := s.topo.CreateProxyInfo(&s.info); err != nil {
		log.PanicErrorf(err, "create proxy node failed")
	}
	if _, err := s.topo.CreateProxyFenceNode(&s.info); err != nil {
		log.PanicErrorf(err, "create fence node failed")
	}
	for {
		info, err := s.topo.GetProxyInfo(s.info.Id)
		if err != nil {
			log.PanicErrorf(err, "get proxy info failed")
		}
		s.info.State = info.State
		switch info.State {
		case models.PROXY_STATE_MARK_OFFLINE:
			log.Infof("action, mark offline")
			s.handleOffline()
			return
		case models.PROXY_STATE_ONLINE:
			log.Infof("we are online: %s", s.info.Id)
			_, err := s.topo.WatchNode(path.Join(models.GetProxyPath(s.topo.ProductName), s.info.Id), s.evtbus)
			if err != nil {
				log.PanicErrorf(err, "watch node failed")
			}
			return
		}
		select {
		case <-s.kill:
			log.Infof("killed, mark offline")
			s.handleOffline()
			return
		default:
		}
		log.Infof("wait to be online: %s", s.info.Id)
		time.Sleep(3 * time.Second)
	}
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

func groupMaster(groupInfo models.ServerGroup) string {
	var master string
	for _, server := range groupInfo.Servers {
		if server.Type == models.SERVER_TYPE_MASTER {
			if master != "" {
				log.Panicf("two master not allowed: %+v", groupInfo)
			}
			master = server.Addr
		}
	}
	if master == "" {
		log.Panicf("master not found: %+v", groupInfo)
	}
	return master
}

func (s *Server) resetSlot(i int) {
	s.router.ResetSlot(i)
}

func (s *Server) fillSlot(i int) {
	slotInfo, slotGroup, err := s.topo.GetSlotByIndex(i)
	if err != nil {
		log.PanicErrorf(err, "get slot by index failed", i)
	}

	var from string
	var addr = groupMaster(*slotGroup)
	if slotInfo.State.Status == models.SLOT_STATUS_MIGRATE {
		fromGroup, err := s.topo.GetGroup(slotInfo.State.MigrateStatus.From)
		if err != nil {
			log.PanicErrorf(err, "get migrate from failed")
		}
		from = groupMaster(*fromGroup)
		if from == addr {
			log.Panicf("set slot %04d migrate from %s to %s", i, from, addr)
		}
	}

	s.groups[i] = slotInfo.GroupId
	s.router.FillSlot(i, addr, from,
		slotInfo.State.Status == models.SLOT_STATUS_PRE_MIGRATE)
}

func (s *Server) onSlotRangeChange(param *models.SlotMultiSetParam) {
	log.Infof("slotRangeChange %+v", param)
	for i := param.From; i <= param.To; i++ {
		switch param.Status {
		case models.SLOT_STATUS_OFFLINE:
			s.resetSlot(i)
		case models.SLOT_STATUS_ONLINE:
			s.fillSlot(i)
		default:
			log.Errorf("can not handle status %v", param.Status)
		}
	}
}

func (s *Server) onGroupChange(groupId int) {
	log.Infof("group changed %d", groupId)
	for i, g := range s.groups {
		if g == groupId {
			s.fillSlot(i)
		}
	}
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
		s.fillSlot(slot.Id)
	case models.ACTION_TYPE_SERVER_GROUP_CHANGED:
		serverGroup := &models.ServerGroup{}
		s.getActionObject(seq, serverGroup)
		s.onGroupChange(serverGroup.Id)
	case models.ACTION_TYPE_SERVER_GROUP_REMOVE:
	//do not care
	case models.ACTION_TYPE_MULTI_SLOT_CHANGED:
		param := &models.SlotMultiSetParam{}
		s.getActionObject(seq, param)
		s.onSlotRangeChange(param)
	default:
		log.Panicf("unknown action %+v", act)
	}
	return true
}

func (s *Server) handleOffline() {
	s.topo.Close(s.info.Id)
}

func (s *Server) processAction(e interface{}) bool {
	if strings.Index(getEventPath(e), models.GetProxyPath(s.topo.ProductName)) == 0 {
		info, err := s.topo.GetProxyInfo(s.info.Id)
		if err != nil {
			log.PanicErrorf(err, "get proxy info failed: %s", s.info.Id)
		}
		if info.State == models.PROXY_STATE_MARK_OFFLINE {
			log.Infof("action, mark offline")
			s.handleOffline()
			return false
		} else {
			_, err := s.topo.WatchNode(path.Join(models.GetProxyPath(s.topo.ProductName), s.info.Id), s.evtbus)
			if err != nil {
				log.PanicErrorf(err, "watch node failed")
			}
			return true
		}
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
		return true
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
		return true
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
	return true
}

func (s *Server) handleTopoEvent() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var tick int = 0
	for {
		select {
		case <-s.kill:
			log.Infof("killed, mark offline")
			s.handleOffline()
			return
		case e := <-s.evtbus:
			evtPath := getEventPath(e)
			log.Infof("got event %s, %v, lastActionSeq %d", s.info.Id, e, s.lastActionSeq)
			if strings.Index(evtPath, models.GetActionResponsePath(s.conf.productName)) == 0 {
				seq, err := strconv.Atoi(path.Base(evtPath))
				if err != nil {
					log.ErrorErrorf(err, "parse action seq failed")
				} else {
					if seq < s.lastActionSeq {
						log.Infof("ignore seq = %d", seq)
						continue
					}
				}
			}
			log.Infof("got event %s, %v, lastActionSeq %d", s.info.Id, e, s.lastActionSeq)
			if !s.processAction(e) {
				return
			}
		case <-ticker.C:
			if maxTick := s.conf.pingPeriod; maxTick != 0 {
				if tick++; tick >= maxTick {
					s.router.KeepAlive()
					tick = 0
				}
			}
		}
	}
}

func (s *Server) Serve() error {
	ch := make(chan net.Conn, 4096)
	defer close(ch)

	go func() {
		for c := range ch {
			x := router.NewSessionSize(c, s.conf.passwd, s.conf.maxBufSize, s.conf.maxTimeout)
			go x.Serve(s.router, s.conf.maxPipeline)
		}
	}()

	for {
		c, err := s.listener.Accept()
		if err != nil {
			return errors.Trace(err)
		}
		ch <- c
	}
}
