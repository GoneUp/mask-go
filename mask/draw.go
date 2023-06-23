package mask

import (
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"os"

	"golang.org/x/image/math/fixed"

	"github.com/flopp/go-findfont"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
)

// GetTextImage generates a bitmap out of a text using the font NotoSans
func GetTextImage(text string) [][]byte {
	fontPath, err := findfont.Find("NotoSans-Regular.ttf")
	if err != nil {
		panic(err)
	}

	// load the font with the freetype library
	fontData, err := ioutil.ReadFile(fontPath)
	if err != nil {
		panic(err)
	}
	font, err := truetype.Parse(fontData)
	if err != nil {
		panic(err)
	}

	//img setup
	size := 14.0
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(font)
	c.SetFontSize(size)

	//setup canvas
	textWidth, _ := getWidthOfString(c, text)
	log.Infof("width: x %s, y %s", textWidth.X, textWidth.Y)
	rect := image.Rect(0, 0, textWidth.X.Ceil(), 16)
	img := image.NewGray(rect)

	//draw real string
	colImg := image.White

	c.SetClip(img.Bounds())
	c.SetDst(img)
	c.SetSrc(colImg)

	pt := freetype.Pt(0, 12)
	endPt, err := c.DrawString(text, pt)
	if err != nil {
		log.Error(err)
		return nil
	}

	log.Info("text pixel len", endPt.X.Ceil())
	out, _ := os.Create("test.png")
	png.Encode(out, img)
	out.Close()

	grayImg := image.NewRGBA(img.Rect)
	binaryMap, _ := getPixels(img, grayImg)
	log.Tracef("gray pixels %v", binaryMap)

	//test write
	out, _ = os.Create("gray.png")
	png.Encode(out, grayImg)
	out.Close()

	return binaryMap
}

// Get the bi-dimensional pixel array
func getPixels(img image.Image, grayImg *image.RGBA) ([][]byte, error) {
	bounds := img.Bounds()
	width, height := bounds.Max.X, 16

	var pixels [][]byte
	for x := 0; x < width; x++ {
		var column []byte
		for y := 0; y < height; y++ {
			log.Trace(img.At(x, y).RGBA())
			r, g, b, _ := img.At(x, y).RGBA()
			gry := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
			var binaryVal byte
			//range up to 65k, this filters out the noise a bit and then set the pixel to 1
			if gry > 25000 {
				binaryVal = 1
			}

			//also have a test img
			if binaryVal == 1 {
				grayImg.Set(x, y, img.At(x, y))
			} else {
				grayImg.Set(x, y, color.Black)
			}
			column = append(column, binaryVal)
		}
		pixels = append(pixels, column)
	}
	return pixels, nil
}

func getWidthOfString(c *freetype.Context, s string) (fixed.Point26_6, error) {
	// nil rectangle is always empty so draw is never called
	c.SetClip(image.Rectangle{})
	p, err := c.DrawString(s, fixed.Point26_6{}) // 0,0
	return p, err
}

// EncodeBitmapForMask converts a bitmap to the custom mask format. Height must be 16 pixel.
func EncodeBitmapForMask(bitmap [][]byte) ([]byte, error) {
	/*
			    column encoded in 2b
		      b1: line 0-7, bit 0-7
		      b2: line 7-15, bit 0-7
	*/

	results := make([]byte, 0)
	for i := range bitmap {
		column := bitmap[i]
		if len(column) != 16 {
			log.Errorf("column %d wrong len %v", i, column)
		}

		var val uint16
		for j := range column {
			if column[j] == 1 {
				switch j {
				case 0:
					val = val | 128

				case 1:
					val = val | 64

				case 2:
					val = val | 32

				case 3:
					val = val | 16

				case 4:
					val = val | 8

				case 5:
					val = val | 4

				case 6:
					val = val | 2

				case 7:
					val = val | 1

				case 8:
					val = val | 32768

				case 9:
					val = val | 16384

				case 10:
					val = val | 8192

				case 11:
					val = val | 4096

				case 12:
					val = val | 2048

				case 13:
					val = val | 1024

				case 14:
					val = val | 512

				case 15:
					val = val | 256

				}
			}
		}

		intBytes := make([]byte, 2)
		binary.LittleEndian.PutUint16(intBytes, val)

		results = append(results, intBytes...)
	}

	return results, nil
}

// EncodeColorArrayForMask envodes a white color array
func EncodeColorArrayForMask(columns int) []byte {
	//white text
	results := make([]byte, 0)
	for i := 0; i < columns; i++ {
		results = append(results, []byte{0xFF, 0xFF, 0xFF}...)
	}
	return results
}
