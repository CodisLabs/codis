// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"fmt"
	"strconv"

	"github.com/docopt/docopt-go"
	log "github.com/ngaut/logging"
	"github.com/wandoulabs/codis/pkg/models"
)

// codis redis instance manage tool

func cmdServer(argv []string) (err error) {
	usage := `usage:
	codis-config server list
	codis-config server add <group_id> <redis_addr> <role>
	codis-config server remove <group_id> <redis_addr>
	codis-config server promote <group_id> <redis_addr>
	codis-config server add-group <group_id>
	codis-config server remove-group <group_id>
`
	args, err := docopt.Parse(usage, argv, true, "", false)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Debug(args)

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
	serverGroup := models.NewServerGroup(globalEnv.ProductName(), groupId)
	var v interface{}
	err := callApi(METHOD_PUT, "/api/server_groups", serverGroup, &v)
	if err != nil {
		return err
	}
	fmt.Println(jsonify(v))
	return nil
}

func runPromoteServerToMaster(groupId int, addr string) error {
	s := models.Server{
		Addr:    addr,
		GroupId: groupId,
	}
	var v interface{}
	err := callApi(METHOD_POST, fmt.Sprintf("/api/server_group/%d/promote", groupId), s, &v)
	if err != nil {
		return err
	}
	fmt.Println(jsonify(v))
	return nil
}

func runAddServerToGroup(groupId int, addr string, role string) error {
	server := models.NewServer(role, addr)
	var v interface{}
	err := callApi(METHOD_PUT, fmt.Sprintf("/api/server_group/%d/addServer", groupId), server, &v)
	if err != nil {
		return err
	}
	fmt.Println(jsonify(v))
	return nil
}

func runListServerGroup() error {
	var v interface{}
	err := callApi(METHOD_GET, "/api/server_groups", nil, &v)
	if err != nil {
		return err
	}
	fmt.Println(jsonify(v))
	return nil
}

func runRemoveServerGroup(groupId int) error {
	var v interface{}
	err := callApi(METHOD_DELETE, fmt.Sprintf("/api/server_group/%d", groupId), nil, &v)
	if err != nil {
		return err
	}
	fmt.Println(jsonify(v))
	return nil
}

func runRemoveServerFromGroup(groupId int, addr string) error {
	var v interface{}
	err := callApi(METHOD_PUT, fmt.Sprintf("/api/server_group/%d/removeServer", groupId), models.Server{Addr: addr}, &v)
	if err != nil {
		return err
	}
	fmt.Println(jsonify(v))
	return nil
}
