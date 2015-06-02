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

	topo "github.com/ngaut/go-zookeeper/zk"
	stats "github.com/ngaut/gostats"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy/group"
	"github.com/wandoulabs/codis/pkg/proxy/router/topology"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

type Server struct {
	slots  [models.DEFAULT_SLOT_NUM]*Slot
	evtbus chan interface{}

	conf *Config
	topo *topology.Topology
	info models.ProxyInfo

	lastActionSeq int

	counter   *stats.Counters
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
	return i >= 0 && i < len(s.slots)
}

func (s *Server) getBackendConn(addr string) *BackendConn {
	for _, slot := range s.slots {
		if slot.bc != nil && slot.bc.Addr == addr {
			return slot.bc
		}
	}
	return NewBackendConn(addr)
}

func (s *Server) putBackendConn(bc *BackendConn) {
	if bc == nil {
		return
	}
	for _, slot := range s.slots {
		if slot.bc == bc {
			return
		}
	}
	bc.Close()
}

func (s *Server) clearSlot(i int) {
	if !s.isValidSlot(i) {
		return
	}
	slot := s.slots[i]
	slot.blockAndWait()

	bc := slot.reset()
	s.putBackendConn(bc)

	slot.unblock()
}

func (s *Server) fillSlot(i int, force bool) {
	if !s.isValidSlot(i) {
		return
	}
	slot := s.slots[i]
	if !force && slot.bc != nil {
		log.Panicf("slot %d already filled, slot: %+v", i, slot)
	}

	slotInfo, slotGroup, err := s.topo.GetSlotByIndex(i)
	if err != nil {
		log.PanicErrorf(err, "get slot by index failed", i)
	}

	var from string
	if slotInfo.State.Status == models.SLOT_STATUS_MIGRATE {
		fromGroup, err := s.topo.GetGroup(slotInfo.State.MigrateStatus.From)
		if err != nil {
			log.PanicErrorf(err, "get migrate from failed")
		}
		from = group.NewGroup(*fromGroup).Master()
	}
	var addr = group.NewGroup(*slotGroup).Master()

	slot.blockAndWait()

	bc := slot.reset()
	s.putBackendConn(bc)

	slot.Info, slot.Group = slotInfo, slotGroup
	slot.from = from
	if len(addr) != 0 {
		xx := strings.Split(addr, ":")
		if len(xx) >= 1 {
			slot.addr.host = []byte(xx[0])
		}
		if len(xx) >= 2 {
			slot.addr.port = []byte(xx[1])
		}
	}
	slot.bc = s.getBackendConn(addr)

	if slotInfo.State.Status != models.SLOT_STATUS_PRE_MIGRATE {
		slot.unblock()
	}

	s.counter.Add("FillSlot", 1)
	log.Infof("fill slot %d, force %v, addr = %s, from = %+v", i, force, addr, from)
}

/*
func (s *Server) handleMigrateState(slotIndex int, key []byte) error {
	panic("todo")
		shd := s.slots[slotIndex]
		if shd.slotInfo.State.Status != models.SLOT_STATUS_MIGRATE {
			return nil
		}

		if shd.migrateFrom == nil {
			log.Fatalf("migrateFrom not exist %+v", shd)
		}

		if shd.dst.Master() == shd.migrateFrom.Master() {
			log.Fatalf("the same migrate src and dst, %+v", shd)
		}

		redisConn, err := s.pools.GetConn(shd.migrateFrom.Master())
		if err != nil {
			return errors.Trace(err)
		}

		defer s.pools.ReleaseConn(redisConn)

		redisReader := redisConn.(*redispool.PooledConn).BufioReader()

		err = WriteMigrateKeyCmd(redisConn.(*redispool.PooledConn), shd.dst.Master(), 30*1000, key)
		if err != nil {
			redisConn.Close()
			log.Warningf("migrate key %s error, from %s to %s",
				string(key), shd.migrateFrom.Master(), shd.dst.Master())
			return errors.Trace(err)
		}

		//handle migrate result
		resp, err := parser.Parse(redisReader)
		if err != nil {
			redisConn.Close()
			return errors.Trace(err)
		}

		result, err := resp.Bytes()

		log.Debug("migrate", string(key), "from", shd.migrateFrom.Master(), "to", shd.dst.Master(),
			string(result))

		if resp.Type == parser.ErrorResp {
			redisConn.Close()
			log.Error(string(key), string(resp.Raw), "migrateFrom", shd.migrateFrom.Master())
			return errors.New(string(resp.Raw))
		}

		s.counter.Add("Migrate", 1)
		return nil
}
*/

/*
func (s *Server) sendBack(c *session, op []byte, keys [][]byte, resp *parser.Resp, result []byte) {
	c.pipelineSeq++
	pr := &PipelineRequest{
		op:    op,
		keys:  keys,
		seq:   c.pipelineSeq,
		backQ: c.backQ,
		req:   resp,
	}

	resp, err := parser.Parse(bufio.NewReader(bytes.NewReader(result)))
	//just send to backQ
	c.backQ <- &PipelineResponse{ctx: pr, err: err, resp: resp}
}

func (s *Server) redisTunnel(c *session) error {
	resp, op, keys, err := getRespOpKeys(c)
	if err != nil {
		return errors.Trace(err)
	}
	k := keys[0]

	opstr := strings.ToUpper(string(op))
	buf, next, err := filter(opstr, keys, c, s.conf.netTimeout)
	if err != nil {
		if len(buf) > 0 { //quit command
			s.sendBack(c, op, keys, resp, buf)
		}
		return errors.Trace(err)
	}

	start := time.Now()
	defer func() {
		recordResponseTime(s.counter, time.Since(start)/1000/1000)
	}()

	s.counter.Add(opstr, 1)
	s.counter.Add("ops", 1)
	if !next {
		s.sendBack(c, op, keys, resp, buf)
		return nil
	}

	if isMulOp(opstr) {
		if len(keys) > 1 { //can not send to redis directly
			var result []byte
			err := s.moper.handleMultiOp(opstr, keys, &result)
			if err != nil {
				return errors.Trace(err)
			}

			s.sendBack(c, op, keys, resp, result)
			return nil
		}
	}

	i := mapKey2Slot(k)

	//pipeline
	c.pipelineSeq++
	pr := &PipelineRequest{
		slotIdx: i,
		op:      op,
		keys:    keys,
		seq:     c.pipelineSeq,
		backQ:   c.backQ,
		req:     resp,
		wg:      &sync.WaitGroup{},
	}
	pr.wg.Add(1)

	s.reqCh <- pr
	pr.wg.Wait()

	return nil
}
*/

/*
 */

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
	done chan error
}

func (s *Server) registerSignal() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, os.Kill)
	go func() {
		<-c
		log.Info("ctrl-c or SIGTERM found, mark offline server")
		done := make(chan error)
		s.evtbus <- &killEvent{done: done}
		<-done
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
			log.Panicf("suicide %+v", s.info)
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

/*
func (s *Server) dispatch(r *PipelineRequest) {
		s.handleMigrateState(r.slotIdx, r.keys[0])
		tr, ok := s.pipeConns[s.slots[r.slotIdx].dst.Master()]
		if !ok {
			//try recreate taskrunner
			if err := s.createTaskRunner(s.slots[r.slotIdx]); err != nil {
				r.backQ <- &PipelineResponse{ctx: r, resp: nil, err: err}
				return
			}

			tr = s.pipeConns[s.slots[r.slotIdx].dst.Master()]
		}
		tr.in <- r
}
*/

func (s *Server) handleTopoEvent() {
	for {
		e := <-s.evtbus
		switch e.(type) {
		case *killEvent:
			s.handleMarkOffline()
			e.(*killEvent).done <- nil
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
				e.(*killEvent).done <- nil
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

func NewServer(addr string, debugVarAddr string, conf *Config) *Server {
	log.Infof("start proxy with config: %+v", conf)
	s := &Server{
		evtbus:        make(chan interface{}, 1000),
		conf:          conf,
		topo:          topology.NewTopo(conf.productName, conf.zkAddr, conf.fact, conf.provider),
		counter:       stats.NewCounters("router"),
		lastActionSeq: -1,
	}
	for i := 0; i < len(s.slots); i++ {
		s.slots[i] = &Slot{Id: i}
	}

	proxyHost := strings.Split(addr, ":")[0]
	if len(proxyHost) != 2 {
		log.Panicf("invalid proxy host = %s", addr)
	}
	debugHost := strings.Split(debugVarAddr, ":")[0]
	if len(debugHost) != 2 {
		log.Panicf("invalid debug host = %s", debugVarAddr)
	}

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

	stats.Publish("evtbus", stats.StringFunc(func() string {
		return strconv.Itoa(len(s.evtbus))
	}))

	l, err := net.Listen(conf.proto, addr)
	if err != nil {
		log.PanicErrorf(err, "open listener %s failed", addr)
	}
	log.Infof("proxy now listening on %s", l.Addr().String())

	s.RegisterAndWait()

	_, err = s.topo.WatchChildren(models.GetWatchActionPath(conf.productName), s.evtbus)
	if err != nil {
		log.PanicErrorf(err, "watch children failed")
	}

	for i := 0; i < len(s.slots); i++ {
		s.fillSlot(i, false)
	}

	go s.handleTopoEvent()

	log.Info("proxy start ok")

	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				log.InfoErrorf(err, "accept conn failed")
			}
			go NewSession(c).Serve()
		}
	}()
	return s
}
