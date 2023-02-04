package mimage

import (
	"image/color"
)

/*
Pattern / Gradient from github.com/fogleman/gg/blob/master/gradient.go
*/

type Pattern interface {
	ColorAt(x, y int) color.Color
}

type Gradient interface {
	Pattern
	AddColorStop(offset float64, color color.Color)
}
