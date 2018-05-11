package models

import (
	"hash/crc32"
	"sort"
	"strconv"
	"sync"
	"strings"
)

const VIRTUAL_NODE_NUMBER = 120


type Node struct {
	Id int			`json:"node_id"`
	Ip string		`json:"node_ip"`
	Weight int      `json:"weight"`
}

func NewNode(id int, server string) *Node {
	serverFields := strings.Split(server, "-")
	addr := serverFields[0]
	weight, _ := strconv.Atoi(serverFields[1])
	return &Node{
		Id: id,
		Ip: addr,
		Weight: weight,
	}
}

type Consistent struct {
	Nodes     map[uint32]Node  `json:"nodes,omitempty"`
	VirtualNum   int			   `json:"virtualNum"`
	NodeStatus map[int]string   `json:"nodeStatus"`
	Ring      HashRing        `json:"ring,omitempty"`
	sync.RWMutex
}

func (c *Consistent) Encode() []byte {
	return jsonEncode(c)
}

func NewConsistent() *Consistent {
	return &Consistent{
		Nodes:     make(map[uint32]Node),
		VirtualNum:   VIRTUAL_NODE_NUMBER,
		NodeStatus: make(map[int]string),
		Ring:      HashRing{},
	}
}

func (c *Consistent) Add(node *Node) bool {
	c.Lock()
	defer c.Unlock()

	if _, ok := c.NodeStatus[node.Id]; ok {
		return false
	}

	count := c.VirtualNum * node.Weight
	for i := 0; i < count; i++ {
		str := c.joinStr(i, node)
		c.Nodes[c.hashStr(str)] = *(node)
	}
	c.NodeStatus[node.Id] = node.Ip+"-"+strconv.Itoa(node.Weight)
	c.sortHashRing()
	return true
}

func (c *Consistent) sortHashRing() {
	c.Ring = HashRing{}
	for k := range c.Nodes {
		c.Ring = append(c.Ring, k)
	}
	sort.Sort(c.Ring)
}

func (c *Consistent) Get(key string) (Node, int) {
	c.RLock()
	defer c.RUnlock()

	hash := c.hashStr(key)

	i := c.search(hash)

	return c.Nodes[c.Ring[i]], i
}

func (c *Consistent) GetNext(i int) (Node, int) {
	c.RLock()
	defer c.RUnlock()

	if i == len(c.Ring)-1 {
		i = 0
	} else {
		i++
	}

	return c.Nodes[c.Ring[i]], i
}

func (c *Consistent) joinStr(i int, node *Node) string {
	return strconv.Itoa(node.Id) +
		"-" + strconv.Itoa(i) +
		"-" + node.Ip +
		"-" + strconv.Itoa(node.Weight)
}

func (c *Consistent) hashStr(key string) uint32 {
	return crc32.ChecksumIEEE([]byte(key))
}

func (c *Consistent) search(hash uint32) int {

	i := sort.Search(len(c.Ring), func(i int) bool { return c.Ring[i] >= hash })
	if i < len(c.Ring) {
		if i == len(c.Ring)-1 {
			return 0
		} else {
			return i
		}
	} else {
		return len(c.Ring) - 1
	}
}

func (c *Consistent) Remove(node *Node) bool {
	c.Lock()
	defer c.Unlock()

	if _, ok := c.NodeStatus[node.Id]; !ok {
		return false
	}

	delete(c.NodeStatus, node.Id)

	count := c.VirtualNum * node.Weight
	for i := 0; i < count; i++ {
		str := c.joinStr(i, node)
		delete(c.Nodes, c.hashStr(str))
	}
	c.sortHashRing()
	return true
}

type HashRing []uint32

func (c HashRing) Len() int {
	return len(c)
}

func (c HashRing) Less(i, j int) bool {
	return c[i] < c[j]
}

func (c HashRing) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}
