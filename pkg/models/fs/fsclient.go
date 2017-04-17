// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package fsclient

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
)

var ErrClosedClient = errors.New("use of closed fs client")

type Client struct {
	sync.Mutex

	RootDir  string
	DataDir  string
	TempDir  string
	LockFile string

	lockfd *os.File
	closed bool
}

func New(dir string) (*Client, error) {
	fullpath, err := filepath.Abs(dir)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &Client{
		RootDir:  fullpath,
		DataDir:  filepath.Join(fullpath, "data"),
		TempDir:  filepath.Join(fullpath, "temp"),
		LockFile: filepath.Join(fullpath, "data.lck"),
	}, nil
}

func (c *Client) realpath(path string) string {
	return filepath.Join(c.DataDir, filepath.Clean(path))
}

func mkdirAll(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func mkdirFor(file string) error {
	dir, _ := filepath.Split(file)
	if dir != "" {
		return mkdirAll(dir)
	}
	return nil
}

func (c *Client) lockFs() error {
	if c.lockfd != nil {
		return errors.Errorf("lock again")
	}
	if err := mkdirFor(c.LockFile); err != nil {
		return err
	}
	f, err := os.OpenFile(c.LockFile, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return errors.Trace(err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return errors.Trace(err)
	}
	var data = map[string]interface{}{
		"pid": os.Getpid(),
		"now": time.Now().String(),
	}
	b, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		log.WarnErrorf(err, "fsclient - lock encode json failed")
	} else if err := f.Truncate(0); err != nil {
		log.WarnErrorf(err, "fsclient - lock truncate failed")
	} else if _, err := f.Write(b); err != nil {
		log.WarnErrorf(err, "fsclient - lock write failed")
	}
	c.lockfd = f
	return nil
}

func (c *Client) unlockFs() {
	if c.lockfd == nil {
		log.Panicf("unlock again")
	}
	var f = c.lockfd
	if err := f.Truncate(0); err != nil {
		log.WarnErrorf(err, "fsclient - unlock truncate failed")
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.WarnErrorf(err, "fsclient - unlock close failed")
		}
	}()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
		log.ErrorErrorf(err, "fsclient - unlock flock failed")
	}
	c.lockfd = nil
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

func (c *Client) newTempFile() (*os.File, error) {
	if err := mkdirAll(c.TempDir); err != nil {
		return nil, err
	}
	prefix := fmt.Sprintf("%d.", int(time.Now().Unix()))
	f, err := ioutil.TempFile(c.TempDir, prefix)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return f, nil
}

func (c *Client) writeFile(realpath string, data []byte, noexists bool) error {
	if noexists {
		_, err := os.Stat(realpath)
		if err == nil {
			return errors.Errorf("file already exists")
		} else if !os.IsNotExist(err) {
			return errors.Trace(err)
		}
	}
	if err := mkdirFor(realpath); err != nil {
		return err
	}

	f, err := c.newTempFile()
	if err != nil {
		return err
	}
	defer f.Close()

	var writeThenRename = func() error {
		_, err := f.Write(data)
		if err != nil {
			return errors.Trace(err)
		}
		if err := f.Close(); err != nil {
			return errors.Trace(err)
		}
		if err := os.Rename(f.Name(), realpath); err != nil {
			return errors.Trace(err)
		}
		return nil
	}
	if err := writeThenRename(); err != nil {
		os.Remove(f.Name())
		return err
	}
	return nil
}

func (c *Client) Create(path string, data []byte) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedClient)
	}

	if err := c.lockFs(); err != nil {
		return err
	}
	defer c.unlockFs()

	if err := c.writeFile(c.realpath(path), data, true); err != nil {
		log.Warnf("fsclient - create %s failed", path)
		return err
	} else {
		log.Infof("fsclient - create %s OK", path)
		return nil
	}
}

func (c *Client) Update(path string, data []byte) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedClient)
	}

	if err := c.lockFs(); err != nil {
		return err
	}
	defer c.unlockFs()

	if err := c.writeFile(c.realpath(path), data, false); err != nil {
		log.Warnf("fsclient - update %s failed", path)
		return err
	} else {
		log.Infof("fsclient - update %s OK", path)
		return nil
	}
}

func (c *Client) Delete(path string) error {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return errors.Trace(ErrClosedClient)
	}

	if err := c.lockFs(); err != nil {
		return err
	}
	defer c.unlockFs()

	if err := os.RemoveAll(c.realpath(path)); err != nil {
		log.Warnf("fsclient - delete %s failed", path)
		return errors.Trace(err)
	} else {
		log.Infof("fsclient - delete %s OK", path)
		return nil
	}
}

func (c *Client) Read(path string, must bool) ([]byte, error) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil, errors.Trace(ErrClosedClient)
	}

	if err := c.lockFs(); err != nil {
		return nil, err
	}
	defer c.unlockFs()

	realpath := c.realpath(path)
	if !must {
		_, err := os.Stat(realpath)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, errors.Trace(err)
			}
			return nil, nil
		}
	}

	b, err := ioutil.ReadFile(realpath)
	if err != nil {
		log.Warnf("fsclient - read %s failed", path)
		return nil, errors.Trace(err)
	}
	return b, nil
}

func (c *Client) List(path string, must bool) ([]string, error) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return nil, errors.Trace(ErrClosedClient)
	}

	if err := c.lockFs(); err != nil {
		return nil, err
	}
	defer c.unlockFs()

	realpath := c.realpath(path)
	if !must {
		_, err := os.Stat(realpath)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, errors.Trace(err)
			}
			return nil, nil
		}
	}

	f, err := os.Open(realpath)
	if err != nil {
		log.Warnf("fsclient - list %s failed", path)
		return nil, errors.Trace(err)
	}
	defer f.Close()

	names, err := f.Readdirnames(-1)
	if err != nil {
		log.Warnf("fsclient - list %s failed", path)
		return nil, errors.Trace(err)
	}
	sort.Strings(names)

	var results []string
	for _, name := range names {
		results = append(results, filepath.Join(path, name))
	}
	return results, nil
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
