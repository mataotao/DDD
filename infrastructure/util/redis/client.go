package redis

import (
	redisgo "github.com/gomodule/redigo/redis"
)

type Client struct {
	pool redisgo.Conn
}

func NewClient(pool redisgo.Conn) *Client {
	c := &Client{
		pool: pool,
	}
	return c
}

func (s *Client) Close() {
	s.pool.Close()
}
