// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	topo "github.com/wandoulabs/codis/pkg/proxy/router/topology"

	"bytes"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy/cachepool"
	"github.com/wandoulabs/codis/pkg/proxy/group"
	"github.com/wandoulabs/codis/pkg/proxy/parser"
	"github.com/wandoulabs/codis/pkg/proxy/redispool"

	"container/list"

	"github.com/juju/errors"
	stats "github.com/ngaut/gostats"
	log "github.com/ngaut/logging"
	"github.com/ngaut/tokenlimiter"
)

type Slot struct {
	slotInfo    *models.Slot
	groupInfo   *models.ServerGroup
	dst         *group.Group
	migrateFrom *group.Group
}

type OnSuicideFun func() error

type Server struct {
	slots  [models.DEFAULT_SLOT_NUM]*Slot
	top    *topo.Topology
	evtbus chan interface{}
	reqCh  chan *PipelineRequest

	lastActionSeq     int
	pi                models.ProxyInfo
	startAt           time.Time
	addr              string
	concurrentLimiter *tokenlimiter.TokenLimiter

	moper       *MultiOperator
	pools       *cachepool.CachePool
	counter     *stats.Counters
	OnSuicide   OnSuicideFun
	netTimeout  int //seconds
	bufferedReq *list.List

	pipeConns map[string]*taskRunner //redis->taskrunner
}

func (s *Server) clearSlot(i int) {
	if !validSlot(i) {
		return
	}

	if s.slots[i] != nil {
		s.slots[i].dst = nil
		s.slots[i].migrateFrom = nil
		s.slots[i] = nil
	}
}

func (s *Server) stopTaskRunners() {
	wg := &sync.WaitGroup{}
	log.Warning("taskrunner count", len(s.pipeConns))
	wg.Add(len(s.pipeConns))
	for _, tr := range s.pipeConns {
		tr.in <- wg
	}
	wg.Wait()

	//remove all
	for k, _ := range s.pipeConns {
		delete(s.pipeConns, k)
	}
}

func (s *Server) fillSlot(i int, force bool) {
	if !validSlot(i) {
		return
	}

	if !force && s.slots[i] != nil { //check
		log.Fatalf("slot %d already filled, slot: %+v", i, s.slots[i])
		return
	}

	log.Infof("fill slot %d, force %v", i, force)

	s.clearSlot(i)

	slotInfo, groupInfo, err := s.top.GetSlotByIndex(i)
	if err != nil {
		log.Fatal(errors.ErrorStack(err))
	}

	slot := &Slot{
		slotInfo:  slotInfo,
		dst:       group.NewGroup(*groupInfo),
		groupInfo: groupInfo,
	}

	s.pools.AddPool(slot.dst.Master())

	if slot.slotInfo.State.Status == models.SLOT_STATUS_MIGRATE {
		//get migrate src group and fill it
		from, err := s.top.GetGroup(slot.slotInfo.State.MigrateStatus.From)
		if err != nil { //todo: retry ?
			log.Fatal(err)
		}
		slot.migrateFrom = group.NewGroup(*from)
		s.pools.AddPool(slot.migrateFrom.Master())
	}

	s.slots[i] = slot
	s.counter.Add("FillSlot", 1)
}

func (s *Server) createTaskRunner(slot *Slot) error {
	dst := slot.dst.Master()
	if _, ok := s.pipeConns[dst]; !ok {
		tr, err := NewTaskRunner(dst, s.netTimeout)
		if err != nil {
			log.Error(dst) //todo: how to handle this error?
			return err
		} else {
			s.pipeConns[dst] = tr
		}
	}

	return nil
}

func (s *Server) createTaskRunners() {
	for _, slotNo := range s.slots {
		s.createTaskRunner(slotNo)
	}
}

func (s *Server) handleMigrateState(slotIndex int, key []byte) error {
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

func (s *Server) filter(opstr string, keys [][]byte, c *session) (rawresp []byte, next bool, err error) {
	if !allowOp(opstr) {
		return nil, false, errors.Trace(fmt.Errorf("%s not allowed", opstr))
	}

	buf, shouldClose, handled, err := handleSpecCommand(opstr, keys, s.netTimeout)
	if shouldClose { //quit command
		return buf, false, errors.Trace(io.EOF)
	}
	if err != nil {
		return nil, false, errors.Trace(err)
	}

	if handled {
		return buf, false, nil
	}

	return nil, true, nil
}

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
	resp, err := parser.Parse(c.r) // read client request
	if err != nil {
		return errors.Trace(err)
	}

	op, keys, err := resp.GetOpKeys()
	if err != nil {
		return errors.Trace(err)
	}

	if len(keys) == 0 {
		keys = [][]byte{[]byte("fakeKey")}
	}

	start := time.Now()
	k := keys[0]

	opstr := strings.ToUpper(string(op))
	//log.Debugf("op: %s, %s", opstr, keys[0])
	buf, next, err := s.filter(opstr, keys, c)
	if err != nil {
		if len(buf) > 0 { //quit command
			s.sendBack(c, op, keys, resp, buf)
		}
		return errors.Trace(err)
	}

	s.counter.Add(opstr, 1)
	s.counter.Add("ops", 1)
	if !next {
		s.sendBack(c, op, keys, resp, buf)
		return nil
	}

	if isMulOp(opstr) {
		if len(keys) > 1 { //can send to redis directly
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
	token := s.concurrentLimiter.Get()
	defer s.concurrentLimiter.Put(token)

	defer func() {
		sec := time.Since(start).Seconds()
		if sec > 2 {
			log.Warningf("op: %s, key:%s, too long %d seconds, client: %s", opstr,
				string(k), int(sec), c.RemoteAddr().String())
		}
		recordResponseTime(s.counter, time.Duration(sec)*1000)
	}()

	//pipeline
	c.pipelineSeq++
	wg := &sync.WaitGroup{}
	wg.Add(1)

	pr := &PipelineRequest{
		slotIdx: i,
		op:      op,
		keys:    keys,
		seq:     c.pipelineSeq,
		backQ:   c.backQ,
		req:     resp,
		wg:      wg,
	}

	s.reqCh <- pr
	wg.Wait()
	return nil
}

func (s *Server) handleConn(c net.Conn) {
	log.Info("new connection", c.RemoteAddr())

	s.counter.Add("connections", 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	client := &session{
		Conn:        c,
		r:           bufio.NewReader(c),
		w:           bufio.NewWriter(c),
		CreateAt:    time.Now(),
		backQ:       make(chan *PipelineResponse, 1000),
		closeSingal: wg,
	}

	go client.WritingLoop()

	var err error

	defer func() {
		wg.Wait() //wait for writer goroutine
		c.Close()
		if err != nil { //todo: fix this ugly error check
			if GetOriginError(err.(*errors.Err)).Error() != io.EOF.Error() {
				log.Warningf("close connection %v, %+v, %v", c.RemoteAddr(), client, errors.ErrorStack(err))
			} else {
				log.Infof("close connection %v, %+v", c.RemoteAddr(), client)
			}
		} else {
			log.Infof("close connection %v, %+v", c.RemoteAddr(), client)
		}

		s.counter.Add("connections", -1)
	}()

	for {
		err = s.redisTunnel(client)
		if err != nil {
			close(client.backQ)
			return
		}
		client.Ops++
	}
}

func (s *Server) OnSlotRangeChange(param *models.SlotMultiSetParam) {
	log.Warningf("slotRangeChange %+v", param)
	if !validSlot(param.From) || !validSlot(param.To) {
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
	log.Warning("group changed", groupId)

	for i, slot := range s.slots {
		if slot.slotInfo.GroupId == groupId {
			s.fillSlot(i, true)
		}
	}
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

func (s *Server) Run() {
	log.Info("listening on", s.addr)
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Warning(errors.ErrorStack(err))
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *Server) responseAction(seq int64) {
	log.Info("send response", seq)
	err := s.top.DoResponse(int(seq), &s.pi)
	if err != nil {
		log.Error(errors.ErrorStack(err))
	}
}

func (s *Server) getProxyInfo() models.ProxyInfo {
	//todo:send request to evtbus, and get response
	var pi = s.pi
	return pi
}

func (s *Server) getActionObject(seq int, target interface{}) {
	act := &models.Action{Target: target}
	err := s.top.GetActionWithSeqObject(int64(seq), act)
	if err != nil {
		log.Fatal(errors.ErrorStack(err))
	}

	log.Infof("%+v", act)
}

func (s *Server) checkAndDoTopoChange(seq int) (needResponse bool) {
	act, err := s.top.GetActionWithSeq(int64(seq))
	if err != nil { //todo: error is not "not exist"
		log.Fatal(errors.ErrorStack(err), "action seq", seq)
	}

	if !StringsContain(act.Receivers, s.pi.Id) { //no need to response
		return false
	}

	s.stopTaskRunners()

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
		log.Fatalf("unknown action %+v", act)
	}

	s.createTaskRunners()

	return true
}

func (s *Server) handleMarkOffline() {
	s.top.Close(s.pi.Id)
	if s.OnSuicide == nil {
		s.OnSuicide = func() error {
			log.Fatalf("suicide %+v", s.pi)
			return nil
		}
	}

	s.OnSuicide()
}

func (s *Server) handleProxyCommand() {
	pi, err := s.top.GetProxyInfo(s.pi.Id)
	if err != nil {
		log.Fatal(errors.ErrorStack(err))
	}

	if pi.State == models.PROXY_STATE_MARK_OFFLINE {
		s.handleMarkOffline()
	}
}

func (s *Server) processAction(e interface{}) {
	actPath := GetEventPath(e)
	if strings.Index(actPath, models.GetProxyPath(s.top.ProductName)) == 0 {
		//proxy event, should be order for me to suicide
		s.handleProxyCommand()
		return
	}

	//re-watch
	nodes, err := s.top.WatchChildren(models.GetWatchActionPath(s.top.ProductName), s.evtbus)
	if err != nil {
		log.Fatal(errors.ErrorStack(err))
	}

	seqs, err := models.ExtraSeqList(nodes)
	if err != nil {
		log.Fatal(errors.ErrorStack(err))
	}

	if len(seqs) == 0 || !s.top.IsChildrenChangedEvent(e) {
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
		log.Warningf("zookeeper restarted or actions were deleted ? lastActionSeq: %d", s.lastActionSeq)
		if s.lastActionSeq > seqs[len(seqs)-1] {
			log.Fatalf("unknown error, zookeeper restarted or actions were deleted ? lastActionSeq: %d, %v", s.lastActionSeq, nodes)
		}

		if s.lastActionSeq == seqs[len(seqs)-1] { //children change or delete event
			return
		}

		//actions node was remove by someone, seems we can handle it
		index = 0
	}

	actions := seqs[index:]
	for _, seq := range actions {
		exist, err := s.top.Exist(path.Join(s.top.GetActionResponsePath(seq), s.pi.Id))
		if err != nil {
			log.Fatal(errors.ErrorStack(err))
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

func (s *Server) handleTopoEvent() {
	for {
		select {
		case r := <-s.reqCh:
			log.Debugf("got event %v, seq:%v", string(r.op), r.seq)
			if s.slots[r.slotIdx].slotInfo.State.Status == models.SLOT_STATUS_PRE_MIGRATE {
				s.bufferedReq.PushBack(r)
				continue
			}

			for e := s.bufferedReq.Front(); e != nil; {
				next := e.Next()
				s.dispatch(e.Value.(*PipelineRequest))
				s.bufferedReq.Remove(e)
				e = next
			}

			s.dispatch(r)
		case e := <-s.evtbus:
			log.Infof("got event %s, %v", s.pi.Id, e)
			switch e.(type) {
			case *killEvent:
				s.handleMarkOffline()
				e.(*killEvent).done <- nil
			default:
				s.processAction(e)
			}
		}
	}
}

func (s *Server) waitOnline() {
	for {
		pi, err := s.top.GetProxyInfo(s.pi.Id)
		if err != nil {
			log.Fatal(errors.ErrorStack(err))
		}

		if pi.State == models.PROXY_STATE_MARK_OFFLINE {
			s.handleMarkOffline()
		}

		if pi.State == models.PROXY_STATE_ONLINE {
			s.pi.State = pi.State
			println("good, we are on line", s.pi.Id)
			log.Info("we are online", s.pi.Id)
			_, err := s.top.WatchNode(path.Join(models.GetProxyPath(s.top.ProductName), s.pi.Id), s.evtbus)
			if err != nil {
				log.Fatal(errors.ErrorStack(err))
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

		println("wait to be online ", s.pi.Id)
		log.Warning(s.pi.Id, "wait to be online")

		time.Sleep(3 * time.Second)
	}
}

func (s *Server) FillSlots() {
	for i := 0; i < models.DEFAULT_SLOT_NUM; i++ {
		s.fillSlot(i, false)
	}
}

func (s *Server) RegisterAndWait() {
	_, err := s.top.CreateProxyInfo(&s.pi)
	if err != nil {
		log.Fatal(errors.ErrorStack(err))
	}

	_, err = s.top.CreateProxyFenceNode(&s.pi)
	if err != nil {
		log.Warning(errors.ErrorStack(err))
	}

	s.registerSignal()
	s.waitOnline()
}

func NewServer(addr string, debugVarAddr string, conf *Conf) *Server {
	log.Infof("start with configuration: %+v", conf)
	s := &Server{
		evtbus:            make(chan interface{}, 100),
		top:               topo.NewTopo(conf.productName, conf.zkAddr, conf.f),
		netTimeout:        conf.netTimeout,
		counter:           stats.NewCounters("router"),
		lastActionSeq:     -1,
		startAt:           time.Now(),
		addr:              addr,
		concurrentLimiter: tokenlimiter.NewTokenLimiter(1000),
		moper:             NewMultiOperator(addr),
		reqCh:             make(chan *PipelineRequest, 1000),
		pools:             cachepool.NewCachePool(),
		pipeConns:         make(map[string]*taskRunner),
		bufferedReq:       list.New(),
	}

	s.pi.Id = conf.proxyId
	s.pi.State = models.PROXY_STATE_OFFLINE
	hname, err := os.Hostname()
	if err != nil {
		log.Fatal("get host name failed", err)
	}
	s.pi.Addr = hname + ":" + strings.Split(addr, ":")[1]
	s.pi.DebugVarAddr = hname + ":" + strings.Split(debugVarAddr, ":")[1]
	log.Infof("proxy_info:%+v", s.pi)
	//todo:fill more field

	stats.Publish("evtbus", stats.StringFunc(func() string {
		return strconv.Itoa(len(s.evtbus))
	}))
	stats.Publish("startAt", stats.StringFunc(func() string {
		return s.startAt.String()
	}))

	s.RegisterAndWait()

	_, err = s.top.WatchChildren(models.GetWatchActionPath(conf.productName), s.evtbus)
	if err != nil {
		log.Fatal(errors.ErrorStack(err))
	}

	s.FillSlots()

	//start event handler
	go s.handleTopoEvent()

	log.Info("proxy start ok")

	return s
}
