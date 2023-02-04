package mimage

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io/ioutil"
	"os"
	"path/filepath"
)

const (
	defaultChunkSize = 500 // pixels (square)
	defaultRoutines  = 4
	metafile         = ".mimage_metadata.json"
)

// Mimage or Massive-Image is intended to handle normal image operations on
// images that are too massive to reasonably fit in memory all at once.
//
// In general the solution here is to buffer chunks of the image to & from disk
// as required, restricting how much of it needs to be in memory at any one time.
// This is, obviously, slower than having the whole image in memory, not least
// because some operations will need to be repeated over discrete chunks.
type Mimage struct {
	bounds image.Rectangle
	cache  *cache

	root      string // path to Mimage files on disk
	chunkSize int
	routines  int
}

// Draw performs a set of bounded write operation(s). Various functions are only
// called on individual chunks when Do() is called.
//
// It is *highly* recommended to only have one ongoing Draw() & Do() call in
// progress at a time. The results of running two operation at the same time
// is not defined.
func (m *Mimage) Draw() Operation {
	return newOperation(m)
}

// Image returns a selected piece of the massive image as an image.
func (m *Mimage) Image(r image.Rectangle) (image.Image, error) {
	dst := image.NewRGBA(r.Sub(r.Min))

	for coord := range m.chunksWithin(r) {
		i, err := m.cache.Load(coord[0], coord[1])
		if err != nil {
			i.Done()
			return dst, err
		}

		img := i.Img.Image()
		draw.Draw(
			dst,
			img.Bounds().Add(image.Pt(coord[0]*m.chunkSize, coord[1]*m.chunkSize)).Sub(r.Min),
			img,
			image.ZP,
			draw.Src,
		)
		i.Done()
	}

	return dst, nil
}

// Mask returns a piece of the massive image to be used as a mask.
func (m *Mimage) Mask(r image.Rectangle) (*image.Alpha, error) {
	dst := image.NewAlpha(r.Sub(r.Min))

	for coord := range m.chunksWithin(r) {
		i, err := m.cache.Load(coord[0], coord[1])
		if err != nil {
			i.Done()
			return dst, err
		}

		msk := i.Img.AsMask()
		draw.Draw(
			dst,
			msk.Bounds().Add(image.Pt(coord[0]*m.chunkSize, coord[1]*m.chunkSize)).Sub(r.Min),
			msk,
			image.ZP,
			draw.Src,
		)
		i.Done()
	}

	return dst, nil
}

// AtOk returns the color in our massive image at (x,y) along with error information
func (m *Mimage) AtOk(x, y int) (color.Color, error) {
	cx, cy, valid := m.toChunk(x, y)
	if !valid {
		return color.RGBA{}, nil
	}

	i, err := m.cache.Load(cx, cy)
	defer i.Done()
	if err != nil {
		return color.RGBA{}, err
	}

	return i.Img.Image().At(x-cx*m.chunkSize, y-cy*m.chunkSize), nil
}

// At returns the color in our massive image at (x,y)
func (m *Mimage) At(x, y int) color.Color {
	c, _ := m.AtOk(x, y)
	return c
}

// Flush ensures that each in memory chunk of the image is written to disk.
func (m *Mimage) Flush() error { return m.cache.Flush() }

// Directory returns the root directory of the current massive image.
func (m *Mimage) Directory() string { return m.root }

// ColorModel returns our native color model. Mostly this means we implement image.Image
func (m *Mimage) ColorModel() color.Model { return color.RGBA64Model }

// Bounds returns the bounds of the massive image
func (m *Mimage) Bounds() image.Rectangle { return m.bounds }

// Width returns the width of the massive image
func (m *Mimage) Width() int { return m.bounds.Max.X - m.bounds.Min.X }

// Height returns the height of the massive image
func (m *Mimage) Height() int { return m.bounds.Max.Y - m.bounds.Min.Y }

// toChunk converts a given (x,y) in the larger image space to a particular image chunk.
func (m *Mimage) toChunk(x, y int) (int, int, bool) {
	cx := x / m.chunkSize
	cy := y / m.chunkSize
	valid := x >= m.bounds.Min.X && x < m.bounds.Max.Y && y >= m.bounds.Min.Y && y < m.bounds.Max.Y
	return cx, cy, valid
}

// chunksWithin returns all chunks within the given rectangle (in the larger image space).
func (m *Mimage) chunksWithin(r image.Rectangle) <-chan [2]int {
	out := make(chan [2]int)

	r = r.Intersect(m.bounds.Sub(image.Pt(1, 1))) // clamp r within bounds

	if r.Empty() { // nothing to do here
		close(out)
		return out
	}

	fx, fy, _ := m.toChunk(r.Min.X, r.Min.Y) // first chunk x,y
	lx, ly, _ := m.toChunk(r.Max.X, r.Max.Y) // last chunk x,y

	go func() {
		for x := fx; x <= lx; x++ {
			for y := fy; y <= ly; y++ {
				out <- [2]int{x, y}
			}
		}
		close(out)
	}()

	return out
}

// New creates a new massive image.
func New(r image.Rectangle, opts ...Option) (*Mimage, error) {
	me := &Mimage{bounds: r, chunkSize: defaultChunkSize, routines: defaultRoutines}
	for _, opt := range opts {
		err := opt(me)
		if err != nil {
			return nil, err
		}
	}

	if me.root == "" {
		// if we don't have a folder, make one
		root, err := os.MkdirTemp("", "mimage")
		if err != nil {
			return nil, err
		}
		me.root = root
	}
	me.cache = newCache(me.root, me.chunkSize)

	// save metadata file
	data, err := encodeJSON(&metadata{
		BoundsMinX: r.Min.X,
		BoundsMinY: r.Min.Y,
		BoundsMaxX: r.Max.X,
		BoundsMaxY: r.Max.Y,
		ChunkSize:  me.chunkSize,
		Routines:   me.routines,
	})
	if err != nil {
		return nil, err
	}
	err = ioutil.WriteFile(filepath.Join(me.root, metafile), data, 0640)

	return me, err
}

// Load a mimage by pointing to it's directory.
func Load(rootdir string) (*Mimage, error) {
	metafile := filepath.Join(rootdir, metafile)

	info, err := os.Stat(metafile)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("expected mimage metadata file got directory %s", metafile)
	}
	data, err := ioutil.ReadFile(metafile)
	if err != nil {
		return nil, err
	}
	meta, err := decodeJSON(data)
	if err != nil {
		return nil, err
	}
	root := filepath.Dir(metafile)
	return &Mimage{
		bounds:    image.Rect(meta.BoundsMinX, meta.BoundsMinY, meta.BoundsMaxX, meta.BoundsMaxY),
		root:      root,
		cache:     newCache(root, meta.ChunkSize),
		chunkSize: meta.ChunkSize,
		routines:  meta.Routines,
	}, nil
}
