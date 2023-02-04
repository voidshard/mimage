package mimage

import (
	"image"
	"image/color"
)

// Operation encodes all functions that we can do on an Mimage
// (which can be over potentially all sub image chunks).
type Operation interface {
	SetFillStyle(g Gradient)
	SetStrokeStyle(g Gradient)
	SetLineWidth(w float64)
	SetColor(c color.Color)
	SetPixel(x, y int)

	MoveTo(x, y float64)
	LineTo(x, y float64)
	ClosePath()

	DrawRectangle(x, y, w, h float64)
	RotateAbout(angle, x, y float64)
	DrawEllipse(x, y, rx, ry float64)

	Fill()
	Stroke()

	Clear()

	SetMask(mask *Mimage)
	InvertMask()
	DrawImage(in image.Image, x, y int)

	// Do performs the given operation.
	//
	// Functions called (above) are performed (in order) across
	// whatever chunk(s) of the image are referenced (determined
	// by bounding calculations on the various (x,y) + (w,h) args
	// passed above as required.
	//
	// That all funcs are called in order is guaranteed. We do not
	// however guarantee the order in which image chunks are
	// read and/or written.
	//
	// Do() returns when all required chunks have been edited,
	// they may not necessarily be flushed from memory (for
	// that see the Flush function). Or when an error is raised.
	Do() error

	// SetRoutines for this operation (defaults to option value
	// given to Mimage on creation).
	SetRoutines(i int)
}
