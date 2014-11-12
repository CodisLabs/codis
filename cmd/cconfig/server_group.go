// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/wandoulabs/codis/pkg/models"

	"github.com/docopt/docopt-go"
	log "github.com/ngaut/logging"
)

// codis redis instance manage tool

func cmdServer(argv []string) (err error) {
	usage := `usage:
	cconfig server list
	cconfig server add <group_id> <redis_addr> <role>
	cconfig server remove <group_id> <redis_addr>
	cconfig server promote <group_id> <redis_addr>
	cconfig server add-group <group_id>
	cconfig server remove-group <group_id>
`
	args, err := docopt.Parse(usage, argv, true, "", false)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Debug(args)

	zkLock.Lock(fmt.Sprintf("server, %+v", argv))
	defer func() {
		err := zkLock.Unlock()
		if err != nil {
			log.Error(err)
		}
	}()

	if args["list"].(bool) {
		return runListServerGroup()
	}

	groupId, err := strconv.Atoi(args["<group_id>"].(string))
	if err != nil {
		log.Warning(err)
		return err
	}

	if args["remove-group"].(bool) {
		return runRemoveServerGroup(groupId)
	}
	if args["add-group"].(bool) {
		return runAddServerGroup(groupId)
	}

	serverAddr := args["<redis_addr>"].(string)
	if args["add"].(bool) {
		role := args["<role>"].(string)
		return runAddServerToGroup(groupId, serverAddr, role)
	}
	if args["remove"].(bool) {
		return runRemoveServerFromGroup(groupId, serverAddr)
	}
	if args["promote"].(bool) {
		return runPromoteServerToMaster(groupId, serverAddr)
	}

	return nil
}

func runAddServerGroup(groupId int) error {
	serverGroup := models.NewServerGroup(productName, groupId)
	if err := serverGroup.Create(zkConn); err != nil {
		return err
	}
	return nil
}

func runPromoteServerToMaster(groupId int, addr string) error {
	group, err := models.GetGroup(zkConn, productName, groupId)
	if err != nil {
		return err
	}

	err = group.Promote(zkConn, addr)
	if err != nil {
		return err
	}
	return nil
}

func runAddServerToGroup(groupId int, addr string, role string) error {
	serverGroup := models.NewServerGroup(productName, groupId)
	if len(addr) > 0 && len(role) > 0 {
		exists, err := serverGroup.Exists(zkConn)
		if err != nil {
			log.Warning(err)
			return err
		}
		// if server group not exists, create it first
		if !exists {
			serverGroup.Create(zkConn)
		}
		server := models.NewServer(role, addr)
		if err := serverGroup.AddServer(zkConn, server); err != nil {
			log.Warning(err)
			return err
		}
	}
	return nil
}

func runListServerGroup() error {
	groups, err := models.ServerGroups(zkConn, productName)
	if err != nil {
		log.Warning(err)
		return err
	}
	b, _ := json.MarshalIndent(groups, " ", "  ")
	fmt.Println(string(b))
	return nil
}

func runRemoveServerGroup(groupId int) error {
	serverGroup := models.NewServerGroup(productName, groupId)
	err := serverGroup.Remove(zkConn)
	if err != nil {
		log.Warning(err)
		return err
	}
	return nil
}

func runRemoveServerFromGroup(groupId int, addr string) error {
	serverGroup, err := models.GetGroup(zkConn, productName, groupId)
	if err != nil {
		log.Warning(err)
		return err
	}
	for _, s := range serverGroup.Servers {
		if s.Addr == addr {
			err := serverGroup.RemoveServer(zkConn, s)
			if err != nil {
				log.Warning(err)
				return err
			}
		}
	}
	return nil
}
