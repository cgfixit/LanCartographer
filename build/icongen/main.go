// Command icongen draws the original LanCartographer mark — a compass rose
// on a navy disc with a single teal accent — and writes it as a Vista-style
// PNG-in-ICO file plus a preview PNG.
//
// It is invoked by //go:generate in cmd/lancartographer (cwd = that package
// dir), so the default outputs land next to the binary sources. From the
// repo root run: go run ./build/icongen -ico cmd/lancartographer/app.ico \
//
//	-png cmd/lancartographer/app_preview.png
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

// palette: one accent color family, unmistakably original
var (
	navy        = color.NRGBA{R: 11, G: 31, B: 51, A: 255}    // #0B1F33 disc
	teal        = color.NRGBA{R: 46, G: 196, B: 182, A: 255}  // #2EC4B6 accent
	tealDim     = color.NRGBA{R: 31, G: 130, B: 121, A: 255}  // #1E8279 diagonals
	ivory       = color.NRGBA{R: 240, G: 244, B: 248, A: 255} // #F0F4F8 north spike
	transparent = color.NRGBA{A: 0}
)

const (
	srcSize = 1024 // render size (4x supersample)
	outSize = 256  // ICO PNG payload size
)

// tri is a triangle; points are unit-circle coordinates (center 0.5,0.5)
type tri struct{ p0, p1, p2 [2]float64 }

func pointInTri(x, y float64, t tri) bool {
	den := (t.p1[1]-t.p2[1])*(t.p0[0]-t.p2[0]) + (t.p2[0]-t.p1[0])*(t.p0[1]-t.p2[1])
	if math.Abs(den) < 1e-12 {
		return false
	}
	a := ((t.p1[1]-t.p2[1])*(x-t.p2[0]) + (t.p2[0]-t.p1[0])*(y-t.p2[1])) / den
	b := ((t.p2[1]-t.p0[1])*(x-t.p2[0]) + (t.p0[0]-t.p2[0])*(y-t.p2[1])) / den
	c := 1 - a - b
	return a >= 0 && b >= 0 && c >= 0
}

// polar converts angle degrees / unit radius to unit-circle coords
func polar(deg, r float64) [2]float64 {
	rad := deg * math.Pi / 180
	return [2]float64{0.5 + r*math.Cos(rad), 0.5 + r*math.Sin(rad)}
}

func main() {
	icoPath := flag.String("ico", "app.ico", "output .ico path (PNG-in-ICO container)")
	pngPath := flag.String("png", "app_preview.png", "output preview .png path")
	flag.Parse()

	img := image.NewNRGBA(image.Rect(0, 0, srcSize, srcSize))

	// compass-rose spikes in unit-circle coords; north is ivory, the rest teal
	baseHalfW, baseY := 0.055, 0.565
	cardinal := []tri{
		{polar(-90, 0.40), [2]float64{0.5 - baseHalfW, baseY}, [2]float64{0.5 + baseHalfW, baseY}},        // N
		{polar(90, 0.34), [2]float64{0.5 - baseHalfW, 1 - baseY}, [2]float64{0.5 + baseHalfW, 1 - baseY}}, // S
		{polar(0, 0.37), [2]float64{1 - baseY, 0.5 - baseHalfW}, [2]float64{1 - baseY, 0.5 + baseHalfW}},  // E
		{polar(180, 0.37), [2]float64{baseY, 0.5 - baseHalfW}, [2]float64{baseY, 0.5 + baseHalfW}},        // W
	}
	var diagonals []tri
	for _, a := range []float64{-45, 45, 135, -135} {
		tip := polar(a, 0.30)
		perp := a + 90
		bx := 0.5 + (tip[0]-0.5)*0.22
		by := 0.5 + (tip[1]-0.5)*0.22
		b1 := [2]float64{bx + 0.014*math.Cos(perp*math.Pi/180), by + 0.014*math.Sin(perp*math.Pi/180)}
		b2 := [2]float64{bx - 0.014*math.Cos(perp*math.Pi/180), by - 0.014*math.Sin(perp*math.Pi/180)}
		diagonals = append(diagonals, tri{tip, b1, b2})
	}

	for py := 0; py < srcSize; py++ {
		for px := 0; px < srcSize; px++ {
			ux := (float64(px) + 0.5) / srcSize
			uy := (float64(py) + 0.5) / srcSize
			dx, dy := ux-0.5, uy-0.5
			r := math.Sqrt(dx*dx + dy*dy)
			c := transparent
			if r <= 0.48 { // navy disc
				c = navy
			}
			if r >= 0.44 && r <= 0.465 { // teal ring on the disc edge
				c = teal
			}
			for _, t := range diagonals {
				if pointInTri(ux, uy, t) {
					c = tealDim
					break
				}
			}
			for _, t := range cardinal[1:] {
				if pointInTri(ux, uy, t) {
					c = teal
					break
				}
			}
			// north spike overpaint (ivory)
			if pointInTri(ux, uy, cardinal[0]) {
				c = ivory
			}
			// center pivot: ivory dot inside teal ring
			if r <= 0.035 {
				c = ivory
			} else if r > 0.035 && r <= 0.045 {
				c = teal
			}
			img.SetNRGBA(px, py, c)
		}
	}

	// box downsample 1024 -> 256
	scale := srcSize / outSize
	out := image.NewNRGBA(image.Rect(0, 0, outSize, outSize))
	for y := 0; y < outSize; y++ {
		for x := 0; x < outSize; x++ {
			var sr, sg, sb, sa float64
			for sy := 0; sy < scale; sy++ {
				for sx := 0; sx < scale; sx++ {
					p := img.NRGBAAt(x*scale+sx, y*scale+sy)
					a := float64(p.A)
					sr += float64(p.R) * a
					sg += float64(p.G) * a
					sb += float64(p.B) * a
					sa += a
				}
			}
			if sa > 0 {
				n := float64(scale * scale)
				out.SetNRGBA(x, y, color.NRGBA{
					R: uint8(sr / sa),
					G: uint8(sg / sa),
					B: uint8(sb / sa),
					A: uint8(sa / n),
				})
			} else {
				out.SetNRGBA(x, y, transparent)
			}
		}
	}

	// encode PNG payload
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, out); err != nil {
		panic(err)
	}
	if err := os.WriteFile(*pngPath, pngBuf.Bytes(), 0o644); err != nil {
		panic(err)
	}

	// write Vista+ PNG-in-ICO container
	ico := new(bytes.Buffer)
	_ = binary.Write(ico, binary.LittleEndian, uint16(0))  // reserved
	_ = binary.Write(ico, binary.LittleEndian, uint16(1))  // type: icon
	_ = binary.Write(ico, binary.LittleEndian, uint16(1))  // count
	ico.WriteByte(0)                                       // width 256
	ico.WriteByte(0)                                       // height 256
	ico.WriteByte(0)                                       // palette colors
	ico.WriteByte(0)                                       // reserved
	_ = binary.Write(ico, binary.LittleEndian, uint16(1))  // planes
	_ = binary.Write(ico, binary.LittleEndian, uint16(32)) // bit count
	_ = binary.Write(ico, binary.LittleEndian, uint32(pngBuf.Len()))
	_ = binary.Write(ico, binary.LittleEndian, uint32(6+16)) // PNG offset
	ico.Write(pngBuf.Bytes())
	if err := os.WriteFile(*icoPath, ico.Bytes(), 0o644); err != nil {
		panic(err)
	}
}
