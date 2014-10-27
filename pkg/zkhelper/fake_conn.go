package zkhelper

import (
	"fmt"
	"path"
	"strings"

	"github.com/ngaut/go-zookeeper/zk"
)

type FakeZkNode struct {
	path     string
	data     []byte
	seq      int
	children []*FakeZkNode
}

type FakeConn struct {
	rootNode *FakeZkNode
}

func NewFakeZkNode(path string, data []byte) *FakeZkNode {
	return &FakeZkNode{
		path: path,
		data: data,
		seq:  -1,
	}
}

func (n *FakeZkNode) GetChildrenWithPath(p string) *FakeZkNode {
	for _, n := range n.children {
		if n.path == p {
			return n
		}
	}
	return nil
}

func NewFakeConn() *FakeConn {
	return &FakeConn{
		rootNode: NewFakeZkNode("/", nil),
	}
}

func (c *FakeConn) getNode(path string) *FakeZkNode {
	dirs := strings.Split(path, "/")
	curNode := c.rootNode
	for _, p := range dirs {
		if len(p) == 0 {
			continue
		}

		curNode = curNode.GetChildrenWithPath(p)
		if curNode == nil {
			return nil
		}
	}
	return curNode
}

func (c *FakeConn) Get(path string) (data []byte, stat *zk.Stat, err error) {
	n := c.getNode(path)
	if n != nil {
		return n.data, nil, nil
	}
	return nil, nil, zk.ErrNoNode
}

func (c *FakeConn) GetW(path string) (data []byte, stat *zk.Stat, watch <-chan zk.Event, err error) {
	return nil, nil, nil, nil
}

func (c *FakeConn) Children(path string) (children []string, stat *zk.Stat, err error) {
	n := c.getNode(path)
	var paths []string = make([]string, 0)
	if n != nil {
		for _, p := range n.children {
			paths = append(paths, p.path)
		}
	}
	return paths, nil, nil
}

func (c *FakeConn) ChildrenW(path string) (children []string, stat *zk.Stat, watch <-chan zk.Event, err error) {
	return nil, nil, nil, nil
}

func (c *FakeConn) Exists(path string) (exist bool, stat *zk.Stat, err error) {
	if n := c.getNode(path); n != nil {
		return true, nil, nil
	}
	return false, nil, nil
}

func (c *FakeConn) ExistsW(path string) (exist bool, stat *zk.Stat, watch <-chan zk.Event, err error) {
	return true, nil, nil, nil
}

func (c *FakeConn) Create(path string, value []byte, flags int32, aclv []zk.ACL) (pathCreated string, err error) {
	dirs := strings.Split(path, "/")
	curNode := c.rootNode
	pa := "/"
	for idx, p := range dirs {
		if len(p) == 0 {
			continue
		}
		child := curNode.GetChildrenWithPath(p)
		if child == nil {
			child = NewFakeZkNode(p, nil)
			if flags&zk.FlagSequence > 0 && idx == len(dirs)-1 {
				max := 0
				for _, child := range curNode.children {
					if child.seq != -1 {
						if child.seq > max {
							max = child.seq
						}
					}
				}
				max += 1
				child.seq = max
				child.path += fmt.Sprintf("%010d", max)
			}
			curNode.children = append(curNode.children, child)
		}
		curNode = child
		pa += child.path + "/"
	}
	curNode.data = value
	return pa[0 : len(pa)-1], nil
}

func (c *FakeConn) Set(path string, value []byte, version int32) (stat *zk.Stat, err error) {
	c.Create(path, value, 0, nil)
	return nil, nil
}

func (c *FakeConn) Delete(p string, version int32) (err error) {
	dir, base := path.Split(p)
	n := c.getNode(dir[0 : len(dir)-1])
	if n != nil {
		for i, v := range n.children {
			if v.path == base {
				s := n.children
				s = append(n.children[:i], n.children[i+1:]...)
				n.children = s
				return nil
			}
		}
	}
	return zk.ErrNoNode
}

func (c *FakeConn) Close() {
}

func (c *FakeConn) GetACL(path string) ([]zk.ACL, *zk.Stat, error) {
	return nil, nil, nil
}

func (c *FakeConn) SetACL(path string, aclv []zk.ACL, version int32) (*zk.Stat, error) {
	return nil, nil
}
