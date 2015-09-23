package topom

import (
	"math"
	"net"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"

	"github.com/wandoulabs/codis/pkg/utils/errors"
)

var ErrFailedRedisClient = errors.New("use of failed redis client")

type RedisClient struct {
	conn redis.Conn
	addr string

	LastErr error
	LastUse time.Time
}

func NewRedisClient(addr string, auth string, timeout time.Duration) (*RedisClient, error) {
	c, err := redis.DialTimeout("tcp", addr, time.Second, timeout, timeout)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if auth != "" {
		_, err := c.Do("AUTH", auth)
		if err != nil {
			c.Close()
			return nil, errors.Trace(err)
		}
	}
	return &RedisClient{
		conn: c, addr: addr, LastUse: time.Now(),
	}, nil
}

func (c *RedisClient) Close() error {
	return c.conn.Close()
}

func (c *RedisClient) command(cmd string, args ...interface{}) (interface{}, error) {
	if c.LastErr != nil {
		return nil, errors.Trace(ErrFailedRedisClient)
	}
	if reply, err := c.conn.Do(cmd, args...); err != nil {
		c.LastErr = errors.Trace(err)
		return nil, c.LastErr
	} else {
		c.LastUse = time.Now()
		return reply, nil
	}
}

func (c *RedisClient) SlotsInfo() (map[int]int, error) {
	if reply, err := c.command("SLOTSINFO"); err != nil {
		return nil, err
	} else {
		infos, err := redis.Values(reply, nil)
		if err != nil {
			return nil, errors.Trace(err)
		}
		slots := make(map[int]int)
		for i, info := range infos {
			p, err := redis.Ints(info, nil)
			if err != nil || len(p) != 2 {
				return nil, errors.Errorf("invalid response[%d] = %v", i, info)
			}
			slots[p[0]] = p[1]
		}
		return slots, nil
	}
}

func (c *RedisClient) SlotsMgrtTagSlot(host string, port string, slotId int) (int, error) {
	if reply, err := c.command("SLOTSMGRTTAGSLOT", host, port, 30*1000, slotId); err != nil {
		return 0, err
	} else {
		p, err := redis.Ints(redis.Values(reply, nil))
		if err != nil || len(p) != 2 {
			return 0, errors.Errorf("invalid response = %v", reply)
		}
		if p[0] != 0 {
			return 0, errors.Errorf("migrate slot-%04d failed, response = %v", slotId, reply)
		}
		return p[1], nil
	}
}

func (c *RedisClient) GetInfo() (map[string]string, error) {
	if reply, err := c.command("INFO"); err != nil {
		return nil, err
	} else {
		text, err := redis.String(reply, nil)
		if err != nil {
			return nil, errors.Trace(err)
		}
		info := make(map[string]string)
		for _, line := range strings.Split(text, "\n") {
			kv := strings.SplitN(line, ":", 2)
			if len(kv) != 2 {
				continue
			}
			if key := strings.TrimSpace(kv[0]); key != "" {
				info[key] = strings.TrimSpace(kv[1])
			}
		}
		return info, nil
	}
}

func (c *RedisClient) GetMaster() (string, error) {
	if info, err := c.GetInfo(); err != nil {
		return "", err
	} else {
		host := info["master_host"]
		port := info["master_port"]
		if host == "" && port == "" {
			return "", nil
		}
		return net.JoinHostPort(host, port), nil
	}
}

func (c *RedisClient) GetMaxMemory() (float64, error) {
	if reply, err := c.command("CONFIG", "GET", "maxmemory"); err != nil {
		return 0, err
	} else {
		p, err := redis.Values(reply, nil)
		if err != nil || len(p) != 2 {
			return 0, errors.Errorf("invalid response = %v", reply)
		}
		v, err := redis.Int(p[1], nil)
		if err != nil {
			return 0, errors.Errorf("invalid response = %v", reply)
		}
		if v != 0 {
			return float64(v), nil
		}
		return math.Inf(0), nil
	}
}

func (c *RedisClient) SlaveOf(master string) error {
	if master == c.addr {
		return errors.Errorf("can not slave of itself")
	}
	if master == "" {
		if _, err := c.command("SLAVEOF", "NO", "ONE"); err != nil {
			return err
		} else {
			return nil
		}
	} else {
		if m, err := c.GetMaster(); err != nil {
			return err
		} else if m == master {
			return nil
		}
		host, port, err := net.SplitHostPort(master)
		if err != nil {
			return errors.Trace(err)
		}
		if _, err := c.command("SLAVEOF", host, port); err != nil {
			return err
		} else {
			return nil
		}
	}
}
