package deepzoom

// Code in this file has been derived from: https://hackernoon.com/in-memory-caching-in-golang

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type NamedDeepZoom struct {
	Id       string
	DeepZoom *DeepZoom
}

type cachedDeepZoom struct {
	NamedDeepZoom
	expireAtTimestamp int64
}

type LocalCache struct {
	stop chan struct{}

	wg        sync.WaitGroup
	mu        sync.RWMutex
	deepzooms map[string]cachedDeepZoom
}

// NewLocalCache Create a new local cache
func NewLocalCache(cleanupInterval time.Duration) *LocalCache {
	log.Info("Creating new cache with cleanup interval ", cleanupInterval)
	lc := &LocalCache{
		deepzooms: make(map[string]cachedDeepZoom),
		stop:      make(chan struct{}),
	}

	lc.wg.Add(1)
	go func(cleanupInterval time.Duration) {
		defer lc.wg.Done()
		lc.cleanupLoop(cleanupInterval)
	}(cleanupInterval)

	return lc
}

// cleanupLoop Cleanup cache and delete / close open slides when cache expired
func (lc *LocalCache) cleanupLoop(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-lc.stop:
			return
		case <-t.C:
			lc.mu.Lock()
			for uid, cu := range lc.deepzooms {
				if cu.expireAtTimestamp <= time.Now().Unix() {
					log.Info("Deepzoom Expired: ", uid)
					// Close underlying slide
					lc.deepzooms[uid].DeepZoom.Slide.Close()
					delete(lc.deepzooms, uid)
				}
			}
			lc.mu.Unlock()
		}
	}
}

func (lc *LocalCache) stopCleanup() {
	close(lc.stop)
	lc.wg.Wait()
}

// Update Add deepzoom to cache
func (lc *LocalCache) Update(u NamedDeepZoom, expireAtTimestamp int64) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	log.Debug(fmt.Sprintf("Updating %s in cache", u.Id))

	lc.deepzooms[u.Id] = cachedDeepZoom{
		NamedDeepZoom:     u,
		expireAtTimestamp: expireAtTimestamp,
	}
	log.Debug(fmt.Sprintf("There are now %s items in cache", len(lc.deepzooms)))
	return nil
}

var (
	errImageNotInCache = errors.New("the deepzoom isn't in cache")
)

// Read Read deepzoom from cache
func (lc *LocalCache) Read(id string) (NamedDeepZoom, error) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	log.Debug("Reading from cache with ID ", id)
	cu, ok := lc.deepzooms[id]
	if !ok {
		log.Debug("ID not found ", id)
		return NamedDeepZoom{}, errImageNotInCache
	}

	return cu.NamedDeepZoom, nil
}

// delete Delete item from cache
func (lc *LocalCache) delete(id string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	// Close underlying slide
	log.Debug("Closing slide with ID ", id)
	lc.deepzooms[id].DeepZoom.Slide.Close()
	delete(lc.deepzooms, id)
}

// EmptyCache Remove all elements from cache and close all file handlers
func (lc *LocalCache) EmptyCache() {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	log.Debug("Emptying complete cache.")
	// delete all elements
	for key, _ := range lc.deepzooms {
		log.Debug(fmt.Sprintf("Deleting key %s", key))
		lc.delete(key)
	}
}
