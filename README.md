# Mimage

Simple library to support operations on massive images with minimal memory.


### Why

I'm working on a world generation library and needing 30G+ of memory to hold a heightmap of a huge world was proving problematic. I was after a simple library similar to the awesome [gg](https://github.com/fogleman/gg) that could deal with massive images via buffering them to & from disk as required. I didn't find one so .. here we are.


### What

Mimage essentially wraps [gg](https://github.com/fogleman/gg) providing many of the same functions, but under the hood the "image" is stored on disk in chunks (by default 500x500 pixels), which are loaded & unloaded from memory as required to apply operations.

This is of course pointless & much slower if you're dealing with small images, but helpful if you want a nice 100kx100k pixel image for some reason. A simple test drawing a rectangle on a 100k pixel square image uses on the order of 3G of memory using Mimage and more like 31G+ and another 10G of swap without it.


### Functions

So far I've added these functions from [gg](https://github.com/fogleman/gg) - they work exactly the same way (or .. should).
```golang
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
    DrawImage(in image.Image, x, y int)
```

The mask functions are a little different
```golang
    SetMask(mask *Mimage) // set an entire mimage as a mask for another
    InvertMask() // inverts the current mask alpha (eg. alpha = 255 - alpha)
```


### How

Simple example

```golang
    // create a new mimage, since we don't give a directory a tmp dir is made for us
    im, _ := mimage.New(image.Rect(0, 0, 1000, 1000))

    // prepare a new operation
    op := im.Draw()

    // draw a red rectangle, these actions are queued up, but nothing happens yet
    op.SetColor(color.RGBA{255, 0, 0, 255})
    op.DrawRectangle(200, 200, 400, 400)
    op.Fill()

    // perform queued operations
    op.Do()

    // flush changes to disk
    im.Flush()
```
Note the final Flush() call; after Do() completes any image chunks not currently being used will be written out eventually, but Flush() ensures this has happened.

In addition to these, the mimage struct itself provides some hopefully helpful functions
```golang
    // return subimage within rectangle
    im.Image(r image.Rectangle) (image.Image, error)

    // return subimage mask within rectangle
    im.Mask(r image.Rectangle) (*image.Alpha, error)

    // to meet the image.Image interface
    At(x, y int)
    ColorModel() color.Model
    Bounds() image.Rectangle

    // returns the color at (x,y) like At() above but with error information (if any)
    AtOk(x, y int) (color.Color, error)
    
    // for those too lazy to calculate from bounds
    Width() int
    Height() int
    
    // the path to the mimage folder on disk
    Directory() string
```



### Notes

- Technically this can support most (all?) functions from [gg](https://github.com/fogleman/gg) these are simply the ones I'm using right now so I added them first.
- You can call multiple operations on the same mimage one after another, but you probably shouldn't have multiple simultaneous operations in progress on the same mimage. What happens in such a case is undefined and will probably be bad.
- The library makes a reasonable guess at which chunks of the massive image need to be loaded in order to honor an operation, but there are edge cases (particularly when drawing lines) that I could improve the effciency of. In general I'd recommend fewer Do() calls with more functions per call than the reverse.
- One directory holds only one mimage, this need not strictly be the case, but it's sort of neater and easier to deal with.
