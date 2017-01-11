// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package dbclient

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"database/sql"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
	_ "github.com/go-sql-driver/mysql"
)

var ErrClosedClient = errors.New("use of closed mysql client")

type Client struct {
	sync.Mutex

	addr    string
	name    string
	user    string
	pwd     string
	timeout time.Duration
	db      *sql.DB

	closed bool
}

func New(addr, name, user, pwd string, timeout time.Duration) (*Client, error) {
	if timeout <= 0 {
		timeout = time.Second * 5
	}
	c := &Client{
		addr: addr, name: name, user: user,
		pwd: pwd, timeout: timeout}
	if err := c.reset(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) reset() error {

	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8", c.user, c.pwd, c.addr, c.name))
	if err != nil {
		return errors.Trace(err)
	}
	if c.db != nil {
		c.db.Close()
	}
	c.db = db

	log.Info("db reset")

	return nil
}

// insert /codis3/codis-demo/topom
// insert /codis3/codis-demo
// insert /codis3

func (c *Client) insert(path string, data []byte) error {

	log.Debugf("insert path:%s,value:%s", path, data)
	fields := strings.Split(path, "/")
	num := len(fields)
	var node_path, node_value, parent_path string
	for index := 1; index < num; index++ {
		if index == 1 {
			parent_path = "/"
		} else {
			parent_path = node_path
		}
		node_path = node_path + "/" + fields[index]
		log.Debugf("insert path:%s,index:%d", node_path, index)
		result, err := c.query(node_path)
		if err != nil {
			return errors.Trace(err)
		}
		if result != nil {
			continue
		}
		stmt, err := c.db.Prepare("INSERT codis SET parent_path=?,path=?,value=?")
		if err != nil {
			return errors.Trace(err)
		}
		if index == num-1 {
			node_value = string(data)
		}
		res, err := stmt.Exec(parent_path, node_path, node_value)
		if err != nil {
			return errors.Trace(err)
		}

		affect, err := res.RowsAffected()
		if err != nil {
			return errors.Trace(err)
		}

		log.Debugf("insert path:%s,parent path:%s,value:%s,affect row:%d", node_path, parent_path, node_value, affect)
	}
	return nil
}

func (c *Client) update(path string, data []byte) error {

	result, err := c.query(path)
	if err != nil {
		return errors.Trace(err)
	}
	if result != nil {
		stmt, err := c.db.Prepare("update codis SET value=? where path =?")
		if err != nil {
			return errors.Trace(err)
		}
		res, err := stmt.Exec(data, path)
		if err != nil {
			return errors.Trace(err)
		}

		affect, err := res.RowsAffected()
		if err != nil {
			return errors.Trace(err)
		}

		log.Debugf("update path:%s,value:%s,affect row:%d", path, data, affect)
		return nil

	} else {

		return c.insert(path, data)
	}
}

func (c *Client) del(path string) error {

	stmt, err := c.db.Prepare("delete from codis where path =?")
	if err != nil {
		return errors.Trace(err)
	}
	res, err := stmt.Exec(path)
	if err != nil {
		return errors.Trace(err)
	}

	affect, err := res.RowsAffected()
	if err != nil {
		return errors.Trace(err)
	}

	log.Debugf("del path:%s,affect row:%d", path, affect)
	return nil
}

func (c *Client) getchildren(path string) ([]string, error) {

	var result []string
	stmt := fmt.Sprintf("select path from codis where parent_path = '%s' ", path)
	log.Debugf("sql is ", stmt)
	rows, err := c.db.Query(stmt)
	if err != nil {
		return nil, errors.Trace(err)
	}
	for rows.Next() {
		var value string
		err := rows.Scan(&value)
		if err != nil {
			return nil, errors.Trace(err)
		}
		result = append(result, value)
	}
	log.Debugf("query children:%s,value:%s", path, result)
	return result, nil
}

func (c *Client) query(path string) ([]string, error) {

	var result []string
	stmt := fmt.Sprintf("select value from codis where path = '%s' ", path)
	log.Debugf("sql is ", stmt)
	rows, err := c.db.Query(stmt)
	if err != nil {
		return nil, errors.Trace(err)
	}
	for rows.Next() {
		var value string
		err := rows.Scan(&value)
		if err != nil {
			return nil, errors.Trace(err)
		}
		result = append(result, value)
	}
	log.Debugf("query path:%s,value:%s", path, result)
	return result, nil
}

func (c *Client) Close() error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return nil
}

func (c *Client) Create(path string, data []byte) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedClient)
	}

	if err := c.insert(path, data); err != nil {
		log.Warnf("db - create %s failed", path)
		return err
	} else {
		log.Infof("db - create %s OK", path)
		return nil
	}
}

func (c *Client) Update(path string, data []byte) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedClient)
	}

	if err := c.update(path, data); err != nil {
		log.Warnf("db - update %s failed", path)
		return err
	} else {
		log.Infof("db - update %s OK", path)
		return nil
	}
}

func (c *Client) Delete(path string) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedClient)
	}

	if err := c.del(path); err != nil {
		log.Warnf("db - delete %s failed", path)
		return errors.Trace(err)
	} else {
		log.Infof("db - delete %s OK", path)
		return nil
	}
}

func (c *Client) Read(path string, must bool) ([]byte, error) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil, errors.Trace(ErrClosedClient)
	}
	var result []string
	var err error
	if result, err = c.query(path); err != nil {
		log.Warnf("db - read %s fail", path)
		return nil, errors.Trace(err)
	} else {
		log.Warnf("db - read %s succ", path)
		if nil == result {
			return nil, errors.Trace(err)
		}
		return []byte(result[0]), errors.Trace(err)
	}
}

func (c *Client) List(path string, must bool) ([]string, error) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil, errors.Trace(ErrClosedClient)
	}

	var result []string
	var err error
	if result, err = c.getchildren(path); err != nil {
		log.Warnf("db - list %s fail", path)
		return nil, errors.Trace(err)
	} else {
		log.Warnf("db - list %s succ", path)
		return result, nil
	}
}

var ErrNotSupported = errors.New("not supported")

func (c *Client) WatchInOrder(path string) (<-chan struct{}, []string, error) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil, nil, errors.Trace(ErrClosedClient)
	}
	return nil, nil, errors.Trace(ErrNotSupported)
}

func (c *Client) CreateEphemeral(path string, data []byte) (<-chan struct{}, error) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil, errors.Trace(ErrClosedClient)
	}
	return nil, errors.Trace(ErrNotSupported)
}

func (c *Client) CreateEphemeralInOrder(path string, data []byte) (<-chan struct{}, string, error) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil, "", errors.Trace(ErrClosedClient)
	}
	return nil, "", errors.Trace(ErrNotSupported)
}
