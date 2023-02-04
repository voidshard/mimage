package mimage

import (
	"image"
	"image/color"
	"math"
	"sync"
)

// deferedFuncID is the id of some function we will call on Do() call(s)
type deferedFuncID int

const (
	setFillStyle = iota
	setStrokeStyle
	setLineWidth
	setColor
	setPixel
	setMask
	invertMask
	moveTo
	lineTo
	closePath
	drawRectangle
	rotateAbout
	drawEllipse
	fill
	stroke
	clear
	drawImage
)

// deferredFunc is a function & arguments to be called on Do()
type deferredFunc struct {
	Func int
	Args []interface{}
}

// newDefFunc returns a deferredFunc struct
func newDefFunc(num int, args ...interface{}) *deferredFunc {
	return &deferredFunc{Func: num, Args: args}
}

// operation represents a set of actions to perform each affected chunk
type operation struct {
	parent *Mimage
	queue  []*deferredFunc

	minX         float64
	minY         float64
	maxX         float64
	maxY         float64
	maxlineWidth float64

	routines int
}

// newOperation returns a new empty operation
func newOperation(parent *Mimage) Operation {
	return &operation{
		parent:   parent,
		queue:    []*deferredFunc{},
		minX:     float64(parent.Width()) + 1,
		minY:     float64(parent.Height()) + 1,
		routines: parent.routines,
	}
}

// Do performs all previously called functions across chunks as required.
func (o *operation) Do() error {
	// clamp down on the area that contains all operations
	o.minX = math.Max(0, o.minX-o.maxlineWidth)
	o.maxX = math.Min(float64(o.parent.Width()), o.maxX+o.maxlineWidth)
	o.minY = math.Max(0, o.minY-o.maxlineWidth)
	o.maxY = math.Min(float64(o.parent.Height()), o.maxY+o.maxlineWidth)

	// channel of chunks we need to change
	work := o.parent.chunksWithin(image.Rect(int(o.minX), int(o.minY), int(o.maxX), int(o.maxY)))

	// standard fan out -> fan in to apply changes to all chunks
	errs := make(chan error)
	wg := &sync.WaitGroup{}

	for i := 0; i < o.routines; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for coords := range work {
				err := o.apply(coords[0], coords[1])
				if err != nil {
					errs <- err
					continue
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(errs)
	}()

	return checkErrors(errs)
}

// apply operation(s) to the given chunk
func (o *operation) apply(chunkX, chunkY int) error {
	ctx, err := o.parent.cache.Load(chunkX, chunkY)
	defer ctx.Done() // whatever happens, unlock chunk
	if err != nil {
		return err
	}

	// offsets for operations, mapping worldspace coords to chunkspace
	offXI, offYI := chunkX*o.parent.chunkSize, chunkY*o.parent.chunkSize
	offX, offY := float64(offXI), float64(offYI)

	// pretty straight forward, apply all operations in order to the chunk with
	// offsets factored in. Since we know all the args that refer to some (x,y) in
	// worldspace we can trivially apply a translation.
	for _, action := range o.queue {
		switch action.Func {
		case setFillStyle:
			ctx.Img.SetFillStyle(action.Args[0].(Gradient))
		case setStrokeStyle:
			ctx.Img.SetStrokeStyle(action.Args[0].(Gradient))
		case setLineWidth:
			ctx.Img.SetLineWidth(action.Args[0].(float64))
		case setColor:
			ctx.Img.SetColor(action.Args[0].(color.Color))
		case setPixel:
			x := action.Args[0].(int) - offXI
			y := action.Args[1].(int) - offYI
			ctx.Img.SetPixel(x, y)
			ctx.setEdited()
		case setMask:
			other := action.Args[0].(*Mimage)
			mbounds := ctx.Img.Image().Bounds()
			mask, err := other.Mask(mbounds.Add(image.Pt(offXI, offYI)))
			if err != nil {
				return err
			}
			ctx.Img.SetMask(mask)
		case invertMask:
			ctx.Img.InvertMask()
		case moveTo:
			x := action.Args[0].(float64) - offX
			y := action.Args[1].(float64) - offY
			ctx.Img.MoveTo(x, y)
		case lineTo:
			x := action.Args[0].(float64) - offX
			y := action.Args[1].(float64) - offY
			ctx.Img.LineTo(x, y)
		case closePath:
			ctx.Img.ClosePath()
		case drawRectangle:
			x := action.Args[0].(float64) - offX
			y := action.Args[1].(float64) - offY
			ctx.Img.DrawRectangle(x, y, action.Args[2].(float64), action.Args[3].(float64))
			ctx.setEdited()
		case rotateAbout:
			x := action.Args[1].(float64) - offX
			y := action.Args[2].(float64) - offY
			ctx.Img.RotateAbout(action.Args[0].(float64), x, y)
		case drawEllipse:
			x := action.Args[0].(float64) - offX
			y := action.Args[1].(float64) - offY
			ctx.Img.DrawEllipse(x, y, action.Args[2].(float64), action.Args[3].(float64))
			ctx.setEdited()
		case fill:
			ctx.Img.Fill()
			ctx.setEdited()
		case stroke:
			ctx.Img.Stroke()
			ctx.setEdited()
		case clear:
			ctx.Img.Clear()
			ctx.setEdited()
		case drawImage:
			i := action.Args[0].(image.Image)
			x := action.Args[1].(int) - offXI
			y := action.Args[2].(int) - offYI
			ctx.Img.DrawImage(i, x, y)
			ctx.setEdited()
		}

	}

	return nil
}

// SetRoutines that will be used for this operation
func (o *operation) SetRoutines(i int) {
	if i < 1 {
		i = 1
	}
	o.routines = i
}

// Draw the image i onto this image, with the top left corner at (x,y).
func (o *operation) DrawImage(i image.Image, x, y int) {
	bnds := i.Bounds()
	o.minMax(float64(x+bnds.Min.X), float64(y+bnds.Min.Y))
	o.minMax(float64(x+bnds.Max.X-bnds.Min.X), float64(y+bnds.Max.Y-bnds.Min.Y))
	o.queue = append(o.queue, newDefFunc(drawImage, i, x, y))
}

// Clear applies the currently set color across the whole image.
// Nb. expensive, obviously.
func (o *operation) Clear() {
	o.queue = append(o.queue, newDefFunc(clear))
	b := o.parent.Bounds()
	o.minX = float64(b.Min.X)
	o.minY = float64(b.Min.Y)
	o.maxX = float64(b.Max.X)
	o.maxY = float64(b.Max.Y)
}

// SetMask allows one to use an Mimage as a mask to another Mimage.
// A mask reduces changes this image by some degree (0-255) based on the
// alpha values on the mask at the same location.
// (Where a value of 255 makes shields the pixel entirely).
func (o *operation) SetMask(mask *Mimage) {
	o.queue = append(o.queue, newDefFunc(setMask, mask))
}

// InvertMask flips the currently set mask's alpha values to be the other
// way around. Ie. higher alpha values become low and vica versa.
func (o *operation) InvertMask() {
	o.queue = append(o.queue, newDefFunc(invertMask))
}

// SetFillStyle configures some gradient to apply to Fill() operations.
func (o *operation) SetFillStyle(g Gradient) {
	o.queue = append(o.queue, newDefFunc(setFillStyle, g))
}

// SetStrokeStyle configures some gradient to apply to Stroke() operations.
func (o *operation) SetStrokeStyle(g Gradient) {
	o.queue = append(o.queue, newDefFunc(setStrokeStyle, g))
}

// SetLineWidth sets the width of the line (see MoveTo, LineTo, Stroke etc).
func (o *operation) SetLineWidth(w float64) {
	w = math.Min(1, w)
	o.maxlineWidth = math.Max(o.maxlineWidth, w)
	o.queue = append(o.queue, newDefFunc(setLineWidth, w))
}

// SetColor sets the color of the 'pen' for lines, SetPixel, Clear etc.
func (o *operation) SetColor(c color.Color) {
	o.queue = append(o.queue, newDefFunc(setColor, c))
}

// SetPixel sets the color at (x,y) to the currently set color.
func (o *operation) SetPixel(x, y int) {
	o.minMax(float64(x), float64(y))
	o.queue = append(o.queue, newDefFunc(setPixel, x, y))
}

// MoveTo moves the pen to (x,y)
func (o *operation) MoveTo(x, y float64) {
	o.minMax(x, y)
	o.queue = append(o.queue, newDefFunc(moveTo, x, y))
}

// LineTo draws (or will draw on stroke) from the current location (see MoveTo)
// to the given (x,y)
func (o *operation) LineTo(x, y float64) {
	o.minMax(x, y)
	o.queue = append(o.queue, newDefFunc(lineTo, x, y))
}

// ClosePath is effectively a LineTo to whatever the first location of the 'pen'
// was when a line was started.
func (o *operation) ClosePath() {
	o.queue = append(o.queue, newDefFunc(closePath))
}

// DrawRectangle draws a rectangle beginning at (x,y) with width w and height h.
func (o *operation) DrawRectangle(x, y, w, h float64) {
	o.minMax(x, y)
	o.minMax(x+w, y+h)
	o.queue = append(o.queue, newDefFunc(drawRectangle, x, y, w, h))
}

// RotateAbout rotates the image around (x,y) by the given angle (radians).
func (o *operation) RotateAbout(angle, x, y float64) {
	o.queue = append(o.queue, newDefFunc(rotateAbout, angle, x, y))
}

// DrawEllipse draws an ellipse at (x,y) with axis lengths of rx, ry
func (o *operation) DrawEllipse(x, y, rx, ry float64) {
	o.minMax(x-rx, y-ry)
	o.minMax(x+rx, y+ry)
	o.queue = append(o.queue, newDefFunc(drawEllipse, x, y, rx, ry))
}

// Fill the queued shape(s) with the currently set color.
func (o *operation) Fill() {
	o.queue = append(o.queue, newDefFunc(fill))
}

// Stroke applies line strokes with the currently set color.
func (o *operation) Stroke() {
	o.queue = append(o.queue, newDefFunc(stroke))
}

// minMax sets internal min & max x & y values
func (o *operation) minMax(x, y float64) {
	o.minX = math.Min(o.minX, x)
	o.maxX = math.Max(o.maxX, x)
	o.minY = math.Min(o.minY, y)
	o.maxY = math.Max(o.maxY, y)
}
