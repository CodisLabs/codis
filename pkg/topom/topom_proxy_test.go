// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"testing"

	"github.com/CodisLabs/codis/pkg/utils/assert"
)

func TestCreateProxy(x *testing.T) {
	t := openTopom()
	defer t.Close()

	check := func(tokens []string) {
		ctx, err := t.newContext()
		assert.MustNoError(err)
		assert.Must(len(ctx.proxy) == len(tokens))
		for _, t := range tokens {
			assert.Must(ctx.proxy[t] != nil)
		}
	}

	p1, c1 := openProxy()
	defer c1.Shutdown()

	check([]string{})
	assert.MustNoError(t.CreateProxy(p1.AdminAddr))
	check([]string{p1.Token})
	assert.Must(t.CreateProxy(p1.AdminAddr) != nil)
	check([]string{p1.Token})

	p2, c2 := openProxy()
	defer c2.Shutdown()

	assert.MustNoError(c2.Shutdown())
	check([]string{p1.Token})
	assert.Must(t.CreateProxy(p2.AdminAddr) != nil)
	check([]string{p1.Token})

	assert.MustNoError(c1.Shutdown())
	check([]string{p1.Token})
}

func TestRemoveProxy(x *testing.T) {
	t := openTopom()
	defer t.Close()

	check := func(tokens []string) {
		ctx, err := t.newContext()
		assert.MustNoError(err)
		assert.Must(len(ctx.proxy) == len(tokens))
		for _, t := range tokens {
			assert.Must(ctx.proxy[t] != nil)
		}
	}

	p1, c1 := openProxy()
	defer c1.Shutdown()

	check([]string{})
	assert.MustNoError(t.CreateProxy(p1.AdminAddr))
	check([]string{p1.Token})
	assert.MustNoError(t.RemoveProxy(p1.Token, false))
	check([]string{})

	p2, c2 := openProxy()
	defer c2.Shutdown()

	assert.MustNoError(t.CreateProxy(p2.AdminAddr))
	check([]string{p2.Token})

	assert.MustNoError(c2.Shutdown())
	assert.Must(t.RemoveProxy(p2.Token, false) != nil)
	check([]string{p2.Token})

	assert.MustNoError(t.RemoveProxy(p2.Token, true))
	check([]string{})
}
