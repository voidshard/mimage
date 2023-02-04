package mimage

import (
	"log"
	"os"
	"sync"
	"time"

	"github.com/fogleman/gg"
)

// context wraps a individual chunk & implements load / unload
// locking etc. In general we simply wrap fogleman's Context.go
// funcs of interest, and leave the actual image manipulation
// up to that.
type context struct {
	key       string
	X, Y      int
	chunkSize int
	edited    bool

	Img      *gg.Context
	loadLock *sync.Mutex

	unloadLock *sync.RWMutex
}

// setEdited means on unload() we have to be written to disk
func (c *context) setEdited() {
	c.edited = true
}

// maybeLoadImage will load the image from disk if required.
func (c *context) maybeLoadImage() error {
	c.loadLock.Lock() // prevent readers all loading the same image
	defer c.loadLock.Unlock()

	if c.Img != nil {
		return nil // it's loaded
	}

	img, err := gg.LoadPNG(c.key)
	if os.IsNotExist(err) {
		c.Img = gg.NewContext(c.chunkSize, c.chunkSize)
		return nil
	} else if err != nil {
		return err
	}

	c.Img = gg.NewContextForImage(img)
	return nil
}

// unloadImage writes an image chunk to disk (if needed) and
// removes the reference to it (switching it to nil).
// If an error occurs we do not remove the image from memory.
func (c *context) unloadImage() error {
	if c.Img == nil {
		return nil // it's not loaded
	}
	if c.edited { // no point writing to disk unless edited
		err := c.Img.SavePNG(c.key)
		if err != nil {
			return err
		}
	}
	c.Img = nil
	return nil
}

// unload loop that continuously attempts to flush in memory chunks
// to disk & unload them *if* they're not currently in use.
// We determine this using a RWLock & the below with() and Done() functions
// (which represent readers telling us "I'm using this chunk!")
//
// If we successfully grab the Lock (Write lock) then readers have to
// wait as we flush this to disk. If another reader wishes to use it then
// it will cause it to be re-loaded with maybeLoadImage().
//
// This approach is somewhat annoying if chunks are randomly accessed
// and we keep unloading image chunks .. but it's less complex than
// using some kind of LRU cache with overarching locking and a more
// advanced 'clean' / 'flush' approach. Or not .. I dunno maybe
// I might try that approach later.
func (c *context) unload() {
	// wake up periodically and flush the image to disk when no one is using it
	var err error
	for {
		time.Sleep(time.Second * 1)
		c.unloadLock.Lock()
		err = c.unloadImage()
		c.unloadLock.Unlock()
		if err != nil {
			log.Println("failed to unload image to disk %s: %v", c.key, err)
		}
	}
}

// With here implies a user wishes to use the image, "please don't unload it"
func (c *context) with() error {
	c.unloadLock.RLock()
	return c.maybeLoadImage()
}

// Done means a user is done with the image, "it can be unloaded"
func (c *context) Done() {
	c.unloadLock.RUnlock()
}

// newContext creates a new context that can be used to access a chunk,
// the actual image doesn't need to exist on disk nor is it read when this
// is called.
func newContext(key string, x, y, chunkSize int) *context {
	c := &context{
		key:        key,
		X:          x,
		Y:          y,
		chunkSize:  chunkSize,
		loadLock:   &sync.Mutex{},
		unloadLock: &sync.RWMutex{},
	}
	return c
}
