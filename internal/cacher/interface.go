package cacher

import (
	"time"

	"github.com/maypok86/otter"
)

type Interface interface {
	Get(string) (*CacheItem, bool)
	Set(string, *CacheItem) bool
	Range(Iter)
}

type Iter = func(string, *CacheItem) bool

func New() (Interface, error) {
	out, err := otter.NewBuilder[string, *CacheItem](1000)
	if err != nil {
		return nil, err
	}

	res, err := out.WithTTL(time.Second * 60).Build()
	if err != nil {
		return nil, err
	}

	return res, nil
}
