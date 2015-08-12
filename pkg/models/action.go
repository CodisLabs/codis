// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
	"github.com/wandoulabs/go-zookeeper/zk"
	"github.com/wandoulabs/zkhelper"
)

type ActionType string

const (
	ACTION_TYPE_SERVER_GROUP_CHANGED ActionType = "group_changed"
	ACTION_TYPE_SERVER_GROUP_REMOVE  ActionType = "group_remove"
	ACTION_TYPE_SLOT_CHANGED         ActionType = "slot_changed"
	ACTION_TYPE_MULTI_SLOT_CHANGED   ActionType = "multi_slot_changed"
	ACTION_TYPE_SLOT_MIGRATE         ActionType = "slot_migrate"
	ACTION_TYPE_SLOT_PREMIGRATE      ActionType = "slot_premigrate"
)

const (
	GC_TYPE_N = iota + 1
	GC_TYPE_SEC
)

type Action struct {
	Type      ActionType  `json:"type"`
	Desc      string      `json:"desc"`
	Target    interface{} `json:"target"`
	Ts        string      `json:"ts"` // timestamp
	Receivers []string    `json:"receivers"`
}

func GetWatchActionPath(productName string) string {
	return fmt.Sprintf("/zk/codis/db_%s/actions", productName)
}

func GetActionResponsePath(productName string) string {
	return path.Join(path.Dir(GetWatchActionPath(productName)), "ActionResponse")
}

func GetActionWithSeq(zkConn zkhelper.Conn, productName string, seq int64, provider string) (*Action, error) {
	var act Action
	data, _, err := zkConn.Get(path.Join(GetWatchActionPath(productName), zkConn.Seq2Str(seq)))
	if err != nil {
		return nil, errors.Trace(err)
	}

	if err := json.Unmarshal(data, &act); err != nil {
		return nil, errors.Trace(err)
	}
	return &act, nil
}

func GetActionObject(zkConn zkhelper.Conn, productName string, seq int64, act interface{}, provider string) error {
	data, _, err := zkConn.Get(path.Join(GetWatchActionPath(productName), zkConn.Seq2Str(seq)))
	if err != nil {
		return errors.Trace(err)
	}

	if err := json.Unmarshal(data, act); err != nil {
		return errors.Trace(err)
	}

	return nil
}

var ErrReceiverTimeout = errors.New("receiver timeout")

func WaitForReceiverWithTimeout(zkConn zkhelper.Conn, productName string, actionZkPath string, proxies []ProxyInfo, timeoutInMs int) error {
	if len(proxies) == 0 {
		return nil
	}

	times := 0
	proxyIds := make(map[string]bool)
	for _, p := range proxies {
		proxyIds[p.Id] = true
	}
	// check every 500ms
	for times < timeoutInMs/500 {
		if times >= 6 && (times*500)%1000 == 0 {
			log.Warnf("abnormal waiting time for receivers: %s %v", actionZkPath, proxyIds)
		}
		// get confirm ids
		nodes, _, err := zkConn.Children(actionZkPath)
		if err != nil {
			return errors.Trace(err)
		}
		for _, node := range nodes {
			id := path.Base(node)
			delete(proxyIds, id)
		}
		if len(proxyIds) == 0 {
			return nil
		}
		times++
		time.Sleep(500 * time.Millisecond)
	}
	log.Warn("proxies didn't responed: ", proxyIds)
	// set offline proxies
	for id, _ := range proxyIds {
		log.Errorf("mark proxy %s to PROXY_STATE_MARK_OFFLINE", id)
		if err := SetProxyStatus(zkConn, productName, id, PROXY_STATE_MARK_OFFLINE); err != nil {
			return errors.Trace(err)
		}
	}
	return ErrReceiverTimeout
}

func GetActionSeqList(zkConn zkhelper.Conn, productName string) ([]int, error) {
	nodes, _, err := zkConn.Children(GetWatchActionPath(productName))
	if err != nil {
		return nil, errors.Trace(err)
	}
	return ExtraSeqList(nodes)
}

func ExtraSeqList(nodes []string) ([]int, error) {
	var seqs []int
	for _, nodeName := range nodes {
		seq, err := strconv.Atoi(nodeName)
		if err != nil {
			return nil, errors.Trace(err)
		}
		seqs = append(seqs, seq)
	}
	sort.Ints(seqs)
	return seqs, nil
}

func ActionGC(zkConn zkhelper.Conn, productName string, gcType int, keep int) error {
	prefix := GetWatchActionPath(productName)
	respPrefix := GetActionResponsePath(productName)

	exists, err := zkhelper.NodeExists(zkConn, prefix)
	if err != nil {
		return errors.Trace(err)
	}
	if !exists {
		// if action path not exists just return nil
		return nil
	}

	actions, _, err := zkConn.Children(prefix)
	if err != nil {
		return errors.Trace(err)
	}

	var act Action
	currentTs := time.Now().Unix()

	if gcType == GC_TYPE_N {
		sort.Strings(actions)
		// keep last 500 actions
		if len(actions)-500 <= keep {
			return nil
		}
		for _, action := range actions[:len(actions)-keep-500] {
			if err := zkhelper.DeleteRecursive(zkConn, path.Join(prefix, action), -1); err != nil {
				return errors.Trace(err)
			}
			err := zkhelper.DeleteRecursive(zkConn, path.Join(respPrefix, action), -1)
			if err != nil && !zkhelper.ZkErrorEqual(err, zk.ErrNoNode) {
				return errors.Trace(err)
			}
		}
	} else if gcType == GC_TYPE_SEC {
		secs := keep
		for _, action := range actions {
			b, _, err := zkConn.Get(path.Join(prefix, action))
			if err != nil {
				return errors.Trace(err)
			}
			if err := json.Unmarshal(b, &act); err != nil {
				return errors.Trace(err)
			}
			log.Infof("action = %s, timestamp = %s", action, act.Ts)
			ts, _ := strconv.ParseInt(act.Ts, 10, 64)

			if currentTs-ts > int64(secs) {
				if err := zkhelper.DeleteRecursive(zkConn, path.Join(prefix, action), -1); err != nil {
					return errors.Trace(err)
				}
				err := zkhelper.DeleteRecursive(zkConn, path.Join(respPrefix, action), -1)
				if err != nil && !zkhelper.ZkErrorEqual(err, zk.ErrNoNode) {
					return errors.Trace(err)
				}
			}
		}
	}
	return nil
}

func CreateActionRootPath(zkConn zkhelper.Conn, path string) error {
	// if action dir not exists, create it first
	exists, err := zkhelper.NodeExists(zkConn, path)
	if err != nil {
		return errors.Trace(err)
	}

	if !exists {
		_, err := zkhelper.CreateOrUpdate(zkConn, path, "", 0, zkhelper.DefaultDirACLs(), true)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func NewAction(zkConn zkhelper.Conn, productName string, actionType ActionType, target interface{}, desc string, needConfirm bool) error {
	// new action with default timeout: 30s
	return NewActionWithTimeout(zkConn, productName, actionType, target, desc, needConfirm, 30*1000)
}

func NewActionWithTimeout(zkConn zkhelper.Conn, productName string, actionType ActionType, target interface{}, desc string, needConfirm bool, timeoutInMs int) error {
	ts := strconv.FormatInt(time.Now().Unix(), 10)

	action := &Action{
		Type:   actionType,
		Desc:   desc,
		Target: target,
		Ts:     ts,
	}

	// set action receivers
	proxies, err := ProxyList(zkConn, productName, func(p *ProxyInfo) bool {
		return p.State == PROXY_STATE_ONLINE
	})
	if err != nil {
		return errors.Trace(err)
	}
	if needConfirm {
		// do fencing here, make sure 'offline' proxies are really offline
		// now we only check whether the proxy lists are match
		fenceProxies, err := GetFenceProxyMap(zkConn, productName)
		if err != nil {
			return errors.Trace(err)
		}
		for _, proxy := range proxies {
			delete(fenceProxies, proxy.Addr)
		}
		if len(fenceProxies) > 0 {
			errMsg := bytes.NewBufferString("Some proxies may not stop cleanly:")
			for k, _ := range fenceProxies {
				errMsg.WriteString(" ")
				errMsg.WriteString(k)
			}
			return errors.Errorf("%s", errMsg)
		}
	}
	for _, p := range proxies {
		buf, err := json.Marshal(p)
		if err != nil {
			return errors.Trace(err)
		}
		action.Receivers = append(action.Receivers, string(buf))
	}

	b, _ := json.Marshal(action)

	prefix := GetWatchActionPath(productName)
	//action root path
	err = CreateActionRootPath(zkConn, prefix)
	if err != nil {
		return errors.Trace(err)
	}

	//response path
	respPath := path.Join(path.Dir(prefix), "ActionResponse")
	err = CreateActionRootPath(zkConn, respPath)
	if err != nil {
		return errors.Trace(err)
	}

	//create response node, etcd do not support create in order directory
	//get path first
	actionRespPath, err := zkConn.Create(respPath+"/", b, int32(zk.FlagSequence), zkhelper.DefaultFileACLs())
	if err != nil {
		log.ErrorErrorf(err, "zk create resp node = %s", respPath)
		return errors.Trace(err)
	}

	//remove file then create directory
	zkConn.Delete(actionRespPath, -1)
	actionRespPath, err = zkConn.Create(actionRespPath, b, 0, zkhelper.DefaultDirACLs())
	if err != nil {
		log.ErrorErrorf(err, "zk create resp node = %s", respPath)
		return errors.Trace(err)
	}

	suffix := path.Base(actionRespPath)

	// create action node
	actionPath := path.Join(prefix, suffix)
	_, err = zkConn.Create(actionPath, b, 0, zkhelper.DefaultFileACLs())
	if err != nil {
		log.ErrorErrorf(err, "zk create action path = %s", actionPath)
		return errors.Trace(err)
	}

	if needConfirm {
		if err := WaitForReceiverWithTimeout(zkConn, productName, actionRespPath, proxies, timeoutInMs); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func ForceRemoveLock(zkConn zkhelper.Conn, productName string) error {
	lockPath := fmt.Sprintf("/zk/codis/db_%s/LOCK", productName)
	children, _, err := zkConn.Children(lockPath)
	if err != nil && !zkhelper.ZkErrorEqual(err, zk.ErrNoNode) {
		return errors.Trace(err)
	}

	for _, c := range children {
		fullPath := path.Join(lockPath, c)
		log.Info("deleting..", fullPath)
		err := zkConn.Delete(fullPath, 0)
		if err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}

func ForceRemoveDeadFence(zkConn zkhelper.Conn, productName string) error {
	proxies, err := ProxyList(zkConn, productName, func(p *ProxyInfo) bool {
		return p.State == PROXY_STATE_ONLINE
	})
	if err != nil {
		return errors.Trace(err)
	}
	fenceProxies, err := GetFenceProxyMap(zkConn, productName)
	if err != nil {
		return errors.Trace(err)
	}
	// remove online proxies's fence
	for _, proxy := range proxies {
		delete(fenceProxies, proxy.Addr)
	}

	// delete dead fence in zookeeper
	path := GetProxyFencePath(productName)
	for remainFence, _ := range fenceProxies {
		fencePath := filepath.Join(path, remainFence)
		log.Info("removing fence: ", fencePath)
		if err := zkhelper.DeleteRecursive(zkConn, fencePath, -1); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}
