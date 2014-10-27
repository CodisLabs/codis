package utils

type Token struct {
}

type TokenLimiter struct {
	count int
	ch    chan *Token
}

func (tl *TokenLimiter) Put(tk *Token) {
	tl.ch <- tk
}

func (tl *TokenLimiter) Get() *Token {
	return <-tl.ch
}

func NewTokenLimiter(count int) *TokenLimiter {
	tl := &TokenLimiter{count: count, ch: make(chan *Token, count)}
	for i := 0; i < count; i++ {
		tl.ch <- &Token{}
	}

	return tl
}
