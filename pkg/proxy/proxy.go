// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy/router"
	"github.com/wandoulabs/codis/pkg/utils/log"
	"github.com/wandoulabs/go-zookeeper/zk"
	topo "github.com/wandoulabs/go-zookeeper/zk"
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
	if proxyHost == "0.0.0.0" || strings.HasPrefix(proxyHost, "127.0.0.") || proxyHost == "" {
		proxyHost = hostname
	}
	if debugHost == "0.0.0.0" || strings.HasPrefix(debugHost, "127.0.0.") || debugHost == "" {
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
		defer s.wait.Done()
		s.serve()
	}()
	return s
}

func (s *Server) SetMyselfOnline() error {
	log.Info("mark myself online")
	info := models.ProxyInfo{
		Id:    s.conf.proxyId,
		State: models.PROXY_STATE_ONLINE,
	}
	b, _ := json.Marshal(info)
	url := "http://" + s.conf.dashboardAddr + "/api/proxy"
	res, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		return errors.New("response code is not 200")
	}
	return nil
}

func (s *Server) serve() {
	defer s.close()

	if !s.waitOnline() {
		return
	}

	s.rewatchNodes()

	for i := 0; i < router.MaxSlotNum; i++ {
		s.fillSlot(i)
	}
	log.Info("proxy is serving")
	go func() {
		defer s.close()
		s.handleConns()
	}()

	s.loopEvents()
}

func (s *Server) handleConns() {
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
			return
		} else {
			ch <- c
		}
	}
}

func (s *Server) Info() models.ProxyInfo {
	return s.info
}

func (s *Server) Join() {
	s.wait.Wait()
}

func (s *Server) Close() error {
	s.close()
	s.wait.Wait()
	return nil
}

func (s *Server) close() {
	s.stop.Do(func() {
		s.listener.Close()
		if s.router != nil {
			s.router.Close()
		}
		close(s.kill)
	})
}

func (s *Server) rewatchProxy() {
	_, err := s.topo.WatchNode(path.Join(models.GetProxyPath(s.topo.ProductName), s.info.Id), s.evtbus)
	if err != nil {
		log.PanicErrorf(err, "watch node failed")
	}
}

func (s *Server) rewatchNodes() []string {
	nodes, err := s.topo.WatchChildren(models.GetWatchActionPath(s.topo.ProductName), s.evtbus)
	if err != nil {
		log.PanicErrorf(err, "watch children failed")
	}
	return nodes
}

func (s *Server) register() {
	if _, err := s.topo.CreateProxyInfo(&s.info); err != nil {
		log.PanicErrorf(err, "create proxy node failed")
	}
	if _, err := s.topo.CreateProxyFenceNode(&s.info); err != nil && err != zk.ErrNodeExists {
		log.PanicErrorf(err, "create fence node failed")
	}
	log.Warn("********** Attention **********")
	log.Warn("You should use `kill {pid}` rather than `kill -9 {pid}` to stop me,")
	log.Warn("or the node resisted on zk will not be cleaned when I'm quiting and you must remove it manually")
	log.Warn("*******************************")
}

func (s *Server) markOffline() {
	s.topo.Close(s.info.Id)
	s.info.State = models.PROXY_STATE_MARK_OFFLINE
}

func (s *Server) waitOnline() bool {
	for {
		info, err := s.topo.GetProxyInfo(s.info.Id)
		if err != nil {
			log.PanicErrorf(err, "get proxy info failed: %s", s.info.Id)
		}
		switch info.State {
		case models.PROXY_STATE_MARK_OFFLINE:
			log.Infof("mark offline, proxy got offline event: %s", s.info.Id)
			s.markOffline()
			return false
		case models.PROXY_STATE_ONLINE:
			s.info.State = info.State
			log.Infof("we are online: %s", s.info.Id)
			s.rewatchProxy()
			return true
		}
		select {
		case <-s.kill:
			log.Infof("mark offline, proxy is killed: %s", s.info.Id)
			s.markOffline()
			return false
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
			log.Panicf("can not handle status %v", param.Status)
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

func (s *Server) processAction(e interface{}) {
	if strings.Index(getEventPath(e), models.GetProxyPath(s.topo.ProductName)) == 0 {
		info, err := s.topo.GetProxyInfo(s.info.Id)
		if err != nil {
			log.PanicErrorf(err, "get proxy info failed: %s", s.info.Id)
		}
		switch info.State {
		case models.PROXY_STATE_MARK_OFFLINE:
			log.Infof("mark offline, proxy got offline event: %s", s.info.Id)
			s.markOffline()
		case models.PROXY_STATE_ONLINE:
			s.rewatchProxy()
		default:
			log.Panicf("unknown proxy state %v", info)
		}
		return
	}

	//re-watch
	nodes := s.rewatchNodes()

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
			//break
			//only handle latest action
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

func (s *Server) loopEvents() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var tick int = 0
	for s.info.State == models.PROXY_STATE_ONLINE {
		select {
		case <-s.kill:
			log.Infof("mark offline, proxy is killed: %s", s.info.Id)
			s.markOffline()
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
			s.processAction(e)
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
