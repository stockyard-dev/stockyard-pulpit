package server

import "github.com/stockyard-dev/stockyard-pulpit/internal/license"

type Limits struct {
	MaxPosts     int
	CustomDomain bool
	Analytics    bool
}

var freeLimits = Limits{
	MaxPosts:     10,
	CustomDomain: false,
	Analytics:    false,
}

var proLimits = Limits{
	MaxPosts:     0,
	CustomDomain: true,
	Analytics:    true,
}

func LimitsFor(info *license.Info) Limits {
	if info != nil && info.IsPro() {
		return proLimits
	}
	return freeLimits
}

func LimitReached(limit, current int) bool {
	if limit == 0 {
		return false
	}
	return current >= limit
}
