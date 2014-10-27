package zkhelper

import (
	"testing"

	"github.com/ngaut/go-zookeeper/zk"
)

func TestCreateNode(t *testing.T) {
	conn := NewFakeConn()

	p := "/zk/codis/test"
	conn.Create(p, []byte("hello"), 0, nil)

	d, _, _ := conn.Get(p)
	if string(d) != "hello" {
		t.Error("node create error")
	}

	if b, _, _ := conn.Exists(p); !b {
		t.Error("node not exists")
	}

	if b, _, _ := conn.Exists("/zk1/codis/test"); b {
		t.Error("node should not exists")
	}

}

func TestSetNode(t *testing.T) {
	conn := NewFakeConn()

	p := "/zk/codis/test"
	conn.Set(p, []byte("hello"), 0)
	d, _, _ := conn.Get(p)

	if string(d) != "hello" {
		t.Error("set error")
	}
}

func TestRemoveNode(t *testing.T) {
	conn := NewFakeConn()

	p := "/zk/codis/test"
	conn.Set(p, []byte("hello"), 0)

	if b, _, _ := conn.Exists(p); !b {
		t.Error("nod set error")
	}

	conn.Delete(p, 0)

	if b, _, _ := conn.Exists(p); b {
		t.Error("node should not exists")
	}
}

func TestChildren(t *testing.T) {
	conn := NewFakeConn()

	p := "/zk/codis/test"
	conn.Set(p, []byte("hello"), 0)
	p2 := "/zk/codis/test2"
	conn.Set(p2, []byte("hello"), 0)

	children, _, _ := conn.Children("/zk/codis")
	if len(children) != 2 {
		t.Error("children error")
	}
}

func TestSeqNode(t *testing.T) {
	conn := NewFakeConn()

	p := "/zk/codis/test/lock-"
	conn.Create(p, []byte("hello"), zk.FlagSequence, nil)
	conn.Create(p, []byte("hello"), zk.FlagSequence, nil)
	conn.Create(p, []byte("hello"), zk.FlagSequence, nil)

	children, _, _ := conn.Children("/zk/codis/test")
	if len(children) != 3 {
		t.Error("num of seq node must be 4")
	}

	b, _, _ := conn.Exists("/zk/codis/test/lock-0000000001")
	if !b {
		t.Error("node not found")
	}
	b, _, _ = conn.Exists("/zk/codis/test/lock-0000000002")
	if !b {
		t.Error("node not found")
	}
	b, _, _ = conn.Exists("/zk/codis/test/lock-0000000003")
	if !b {
		t.Error("node not found")
	}
}
