package proxy

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/CodisLabs/codis/pkg/utils/errors"

	redigo "github.com/garyburd/redigo/redis"
)

type Sentinel struct {
	ProductName string
	ProductAuth string

	Address string

	conn redigo.Conn
}

func (s *Sentinel) MasterName(group int) string {
	return fmt.Sprintf("%s-%d", s.ProductName, group)
}

func (s *Sentinel) build() (redigo.Conn, error) {
	if s.conn != nil && s.conn.Err() != nil {
		s.Close()
	}
	if s.conn != nil {
		return s.conn, nil
	}
	c, err := redigo.Dial("tcp", s.Address, []redigo.DialOption{
		redigo.DialConnectTimeout(time.Second * 5),
		redigo.DialReadTimeout(time.Minute), redigo.DialWriteTimeout(time.Second * 10),
	}...)
	if err != nil {
		return nil, errors.Trace(err)
	} else {
		s.conn = c
		return s.conn, nil
	}
}

func (s *Sentinel) Close() {
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
}

func (s *Sentinel) MonitorMaster(group int, address string, quorum int) error {
	c, err := s.build()
	if err != nil {
		return err
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return errors.Trace(err)
	}

	var master = s.MasterName(group)
	if _, err := c.Do("SENTINEL", "monitor", master, host, port, quorum); err != nil {
		return errors.Trace(err)
	}
	if s.ProductAuth != "" {
		_, err := c.Do("SENTINEL", "auth-pass", master, s.ProductAuth)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (s *Sentinel) MonitorRemove(group int) error {
	c, err := s.build()
	if err != nil {
		return err
	}

	var master = s.MasterName(group)
	if _, err := c.Do("SENTINEL", "remove", master); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (s *Sentinel) Masters(groups ...int) (map[int]string, error) {
	c, err := s.build()
	if err != nil {
		return nil, err
	}

	var masters = make(map[int]string, len(groups))
	for i, group := range groups {
		r, err := redigo.Strings(c.Do("SENTINEL", "get-master-addr-by-name", s.MasterName(group)))
		if err != nil {
			return nil, errors.Trace(err)
		}
		switch len(r) {
		case 0:
		case 2:
			masters[i] = fmt.Sprintf("%s:%s", r[0], r[1])
		default:
			return nil, errors.Errorf("invalid response = %v", r)
		}
	}
	return masters, nil
}

func (s *Sentinel) WaitSwitchMaster() error {
	c, err := s.build()
	if err != nil {
		return err
	}

	if err := c.Send("SUBSCRIBE", "+switch-master"); err != nil {
		return errors.Trace(err)
	}
	if err := c.Flush(); err != nil {
		return errors.Trace(err)
	}
	for {
		r, err := redigo.Strings(c.Receive())
		if err != nil {
			return errors.Trace(err)
		}
		if len(r) != 3 {
			return errors.Errorf("invalid response = %v", r)
		}
		if strings.HasPrefix(r[2], s.ProductName) {
			return nil
		}
	}
}
