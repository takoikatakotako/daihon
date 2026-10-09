package render

import "image"

// treatBackdrop dims and blurs a drawn background in place.
//
// dim is 0..1, the fraction of the way to black. blur is a gaussian radius in
// pixels, approximated by three box blurs: three passes of radius r give a
// standard deviation close to r, and each pass is a running sum, so the cost
// is linear in the pixel count and independent of the radius. The static
// layer is cached per background change, so this runs a handful of times per
// video, not per frame.
func treatBackdrop(img *image.RGBA, dim, blur float64) {
	if r := int(blur + 0.5); r > 0 {
		tmp := image.NewRGBA(img.Bounds())
		for range 3 {
			boxBlurH(tmp, img, r)
			boxBlurV(img, tmp, r)
		}
	}
	if dim > 0 {
		if dim > 1 {
			dim = 1
		}
		keep := uint32((1 - dim) * 256)
		pix := img.Pix
		for i := 0; i+3 < len(pix); i += 4 {
			pix[i] = uint8(uint32(pix[i]) * keep >> 8)
			pix[i+1] = uint8(uint32(pix[i+1]) * keep >> 8)
			pix[i+2] = uint8(uint32(pix[i+2]) * keep >> 8)
		}
	}
}

// boxBlurH averages each pixel with its r neighbours on either side along the
// row, clamping at the edges. dst and src must share bounds.
func boxBlurH(dst, src *image.RGBA, r int) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	for y := range h {
		row := src.Pix[y*src.Stride : y*src.Stride+w*4]
		out := dst.Pix[y*dst.Stride : y*dst.Stride+w*4]
		blurLine(out, row, 4, w, r)
	}
}

// boxBlurV is boxBlurH along columns.
func boxBlurV(dst, src *image.RGBA, r int) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	col := make([]uint8, h*4)
	out := make([]uint8, h*4)
	for x := range w {
		for y := range h {
			copy(col[y*4:y*4+4], src.Pix[y*src.Stride+x*4:y*src.Stride+x*4+4])
		}
		blurLine(out, col, 4, h, r)
		for y := range h {
			copy(dst.Pix[y*dst.Stride+x*4:y*dst.Stride+x*4+4], out[y*4:y*4+4])
		}
	}
}

// blurLine box-blurs n pixels of `ch` channels each, with a running sum per
// channel and edge pixels repeated beyond the ends.
func blurLine(out, in []uint8, ch, n, r int) {
	if n == 0 {
		return
	}
	if r >= n {
		r = n - 1
	}
	div := uint32(2*r + 1)
	px := func(i int) []uint8 {
		if i < 0 {
			i = 0
		} else if i >= n {
			i = n - 1
		}
		return in[i*ch : i*ch+ch]
	}
	var sum [4]uint32
	for i := -r; i <= r; i++ {
		p := px(i)
		for c := range ch {
			sum[c] += uint32(p[c])
		}
	}
	for i := range n {
		for c := range ch {
			out[i*ch+c] = uint8((sum[c] + div/2) / div)
		}
		add, sub := px(i+r+1), px(i-r)
		for c := range ch {
			sum[c] += uint32(add[c]) - uint32(sub[c])
		}
	}
}
