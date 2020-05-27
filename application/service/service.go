package service

import (
	"math/rand"
	"time"
)

const (
	OneDayTime int = 86400
)

type service struct{}

func (s *service) cacheTimes(base int) int {
	rand.Seed(time.Now().Unix())
	return base + rand.Intn(3600)
}
