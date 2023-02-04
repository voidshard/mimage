package mimage

import (
	"fmt"
	"path/filepath"
	"sync"
)

// cache is a simple struct to help enforce we only have one
// of any given chunk loaded at a time.
type cache struct {
	root      string
	chunkLock *sync.Mutex
	chunks    map[string]*context
	chunkSize int
}

// newCache prepares a new mimage chunk cache
func newCache(root string, chunkSize int) *cache {
	c := &cache{
		root:      root,
		chunkLock: &sync.Mutex{},
		chunks:    map[string]*context{},
		chunkSize: chunkSize,
	}
	return c
}

// Flush writes all in memory chunks to disk.
// All chunks are locked, then flushed, and finally unlocked.
// It is expected that you're done writing when this is called.
func (c *cache) Flush() error {
	c.chunkLock.Lock()
	defer c.chunkLock.Unlock()

	// lock everything
	for _, ctx := range c.chunks {
		ctx.unloadLock.Lock()
		defer ctx.unloadLock.Unlock()
	}

	// write everything
	for _, ctx := range c.chunks {
		err := ctx.unloadImage()
		if err != nil {
			return err
		}
	}

	return nil
}

// Load a chunk by its x-y coords.
//
// Any chunks returned this way should have Done() called on them
// when the user no longer needs them in memory.
func (c *cache) Load(x, y int) (*context, error) {
	// TODO: we probably can work with other image types
	key := filepath.Join(c.root, fmt.Sprintf("%d.%d.png", x, y))

	c.chunkLock.Lock()

	ctx, ok := c.chunks[key]
	if ok {
		c.chunkLock.Unlock()
		return ctx, ctx.with()
	}

	ctx = newContext(key, x, y, c.chunkSize)
	c.chunks[key] = ctx
	err := ctx.with()
	c.chunkLock.Unlock()

	go ctx.unload()
	return ctx, err
}
