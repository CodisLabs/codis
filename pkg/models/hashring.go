package models

import (
	"hash/crc32"
	"sort"
	"strconv"
	"sync"
	"fmt"
	"strings"
)

const DEFAULT_REPLICAS = 200

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

type Node struct {
	Id int
	Ip string
	//Port     int
	//HostName string
	Weight int
}

func NewNode(id int, server string) *Node {
	serverFields := strings.Split(server, "-")
	addr := serverFields[0]
	weight, _ := strconv.Atoi(serverFields[1])
	fmt.Println("addr:", addr)
	fmt.Println("weight:", weight)
	return &Node{
		Id: id,
		Ip: addr,
		//Port:     port,
		//HostName: name,
		Weight: weight,
	}
}

type Consistent struct {
	Nodes     map[uint32]Node
	numReps   int
	Resources map[int]bool
	ring      HashRing
	sync.RWMutex
}

func NewConsistent() *Consistent {
	return &Consistent{
		Nodes:     make(map[uint32]Node),
		numReps:   DEFAULT_REPLICAS,
		Resources: make(map[int]bool),
		ring:      HashRing{},
	}
}

func (c *Consistent) Add(node *Node) bool {
	c.Lock()
	defer c.Unlock()

	if _, ok := c.Resources[node.Id]; ok {
		return false
	}

	count := c.numReps * node.Weight
	for i := 0; i < count; i++ {
		str := c.joinStr(i, node)
		c.Nodes[c.hashStr(str)] = *(node)
	}
	c.Resources[node.Id] = true
	c.sortHashRing()
	return true
}

func (c *Consistent) sortHashRing() {
	c.ring = HashRing{}
	for k := range c.Nodes {
		c.ring = append(c.ring, k)
	}
	sort.Sort(c.ring)
}

func (c *Consistent) joinStr(i int, node *Node) string {
	//return node.Ip + "*" + strconv.Itoa(node.Weight) +
	//	"-" + strconv.Itoa(i) +
	//	"-" + strconv.Itoa(node.Id)
	return strconv.Itoa(node.Id) +
		"-" + strconv.Itoa(i) +
		"-" + node.Ip +
		"-" + strconv.Itoa(node.Weight)
}

// MurMurHash算法 :https://github.com/spaolacci/murmur3
func (c *Consistent) hashStr(key string) uint32 {
	return crc32.ChecksumIEEE([]byte(key))
}

func (c *Consistent) Get(key string) (Node, int) {
	c.RLock()
	defer c.RUnlock()

	hash := c.hashStr(key)
	fmt.Println("key:", key)

	i := c.search(hash)
	fmt.Println("i:", i)
	fmt.Println("node id:", c.Nodes[c.ring[i]].Id)
	fmt.Println("len(c.ring):", len(c.Nodes))

	return c.Nodes[c.ring[i]], i
}

func (c *Consistent) GetNext(i int) (Node, int) {
	c.RLock()
	defer c.RUnlock()

	//hash := c.hashStr(key)
	//fmt.Println("key:", key)
	//
	//i := c.search(hash)
	if i == len(c.ring)-1 {
		i = 0
	} else {
		i++
	}

	return c.Nodes[c.ring[i]], i
}

func (c *Consistent) search(hash uint32) int {

	i := sort.Search(len(c.ring), func(i int) bool { return c.ring[i] >= hash })
	if i < len(c.ring) {
		if i == len(c.ring)-1 {
			return 0
		} else {
			return i
		}
	} else {
		return len(c.ring) - 1
	}
}

func (c *Consistent) Remove(node *Node) bool {
	c.Lock()
	defer c.Unlock()

	if _, ok := c.Resources[node.Id]; !ok {
		return false
	}

	delete(c.Resources, node.Id)

	count := c.numReps * node.Weight
	for i := 0; i < count; i++ {
		str := c.joinStr(i, node)
		delete(c.Nodes, c.hashStr(str))
	}
	c.sortHashRing()
	return true
}

func main() {

	cHashRing := NewConsistent()

	for i := 0; i < 10; i++ {
		//si := fmt.Sprintf("%d", i)
		cHashRing.Add(NewNode(i, "10.202.94."+strconv.Itoa(i)+"-1"))
	}

	for k, v := range cHashRing.Nodes {
		fmt.Println("Hash:", k, " Id:", v.Id)
	}

	idMap := make(map[int]int, 0)
	for i := 0; i < 1024; i++ {
		si := fmt.Sprintf("slot%d", i)
		k, _ := cHashRing.Get(si)
		if _, ok := idMap[k.Id]; ok {
			idMap[k.Id] += 1
		} else {
			idMap[k.Id] = 1
		}
	}

	sum := 0

	for k, v := range idMap {
		fmt.Println("Node Id:", k, " count:", v)
		sum += v
	}

}
