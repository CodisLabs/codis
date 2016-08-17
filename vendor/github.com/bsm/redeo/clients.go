package redeo

import "sync"

type clients struct {
	m map[uint64]*Client
	l sync.Mutex
}

func newClientRegistry() *clients {
	return &clients{
		m: make(map[uint64]*Client, 10),
	}
}

// Put adds a client connection
func (c *clients) Put(client *Client) {
	c.l.Lock()
	c.m[client.id] = client
	c.l.Unlock()
}

// Close removes a client connection
func (c *clients) Close(id uint64) error {
	c.l.Lock()
	client, ok := c.m[id]
	delete(c.m, id)
	c.l.Unlock()

	if ok {
		return client.close()
	}
	return nil
}

// Clear closes all client connections
func (c *clients) Clear() (err error) {
	c.l.Lock()
	defer c.l.Unlock()

	for id, conn := range c.m {
		if e := conn.close(); e != nil {
			err = e
		}
		delete(c.m, id)
	}
	return
}

// Len returns the length
func (c *clients) Len() int {
	c.l.Lock()
	n := len(c.m)
	c.l.Unlock()
	return n
}

// All creates a snapshot of clients
func (c *clients) All() []*Client {
	c.l.Lock()
	defer c.l.Unlock()

	slice := make([]*Client, 0, len(c.m))
	for _, c := range c.m {
		slice = append(slice, c)
	}
	return slice
}
