//go:build unit

package service

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/stretchr/testify/require"
)

func TestAdobeGIFBillingUsesDeliveredFrameDimensions(t *testing.T) {
	var images [][]byte
	for _, tc := range []struct {
		edge uint16
		want adobe.OutputResolution
	}{{512, adobe.Resolution1K}, {1536, adobe.Resolution2K}, {3072, adobe.Resolution4K}} {
		data := adobeTestGIF(t)
		binary.LittleEndian.PutUint16(data[6:8], tc.edge)
		binary.LittleEndian.PutUint16(data[8:10], tc.edge)
		_, err := gif.DecodeAll(bytes.NewReader(data))
		require.NoError(t, err)
		tier, err := adobeGIFOutputResolution([][]byte{data})
		require.NoError(t, err)
		require.Equal(t, tc.want, tier)
		images = append(images, data)
	}
	tier, err := adobeGIFOutputResolution(images)
	require.NoError(t, err)
	require.Equal(t, adobe.Resolution4K, tier)
	_, err = adobeGIFOutputResolution([][]byte{[]byte("GIF89a")})
	require.Error(t, err)
}

func TestNormalizeAdobeOutputFormat(t *testing.T) {
	pngFormat, err := normalizeAdobeOutputFormat(" PNG ")
	require.NoError(t, err)
	require.Equal(t, adobeOutputFormatPNG, pngFormat)

	jpegFormat, err := normalizeAdobeOutputFormat("JPG")
	require.NoError(t, err)
	require.Equal(t, adobeOutputFormatJPEG, jpegFormat)

	empty, err := normalizeAdobeOutputFormat("")
	require.NoError(t, err)
	require.Empty(t, empty)

	_, err = normalizeAdobeOutputFormat("webp")
	require.ErrorContains(t, err, "output_format=")
	gifFormat, err := normalizeAdobeOutputFormat(" GIF ")
	require.NoError(t, err)
	require.Equal(t, adobeOutputFormatGIF, gifFormat)
}

func TestTranscodeAdobeImagePNGIdentity(t *testing.T) {
	src := adobeTestPNG(t, false)
	got, err := transcodeAdobeImage(src, adobeOutputFormatPNG, intPtr(50))
	require.NoError(t, err)
	require.Equal(t, src, got, "PNG→PNG must not decode/re-encode")
}

func TestTranscodeAdobeImageJPEGToPNG(t *testing.T) {
	src := adobeTestJPEG(t, 90)
	got, err := transcodeAdobeImage(src, adobeOutputFormatPNG, nil)
	require.NoError(t, err)
	require.Equal(t, adobeOutputFormatPNG, sniffAdobeImageFormat(got))
	cfg, _, err := image.DecodeConfig(bytes.NewReader(got))
	require.NoError(t, err)
	require.Equal(t, 8, cfg.Width)
	require.Equal(t, 8, cfg.Height)
}

func TestTranscodeAdobeImageOpaquePNGToJPEG(t *testing.T) {
	src := adobeTestPNG(t, false)
	got, err := transcodeAdobeImage(src, adobeOutputFormatJPEG, nil)
	require.NoError(t, err)
	require.Equal(t, adobeOutputFormatJPEG, sniffAdobeImageFormat(got))
}

func TestTranscodeAdobeImageTransparentPNGToJPEGRejected(t *testing.T) {
	src := adobeTestPNG(t, true)
	_, err := transcodeAdobeImage(src, adobeOutputFormatJPEG, nil)
	require.ErrorContains(t, err, "transparent")
}

func TestApplyAdobeOutputFormatFallsBackOnCorruptBytes(t *testing.T) {
	src := []byte("not-an-image")
	got := applyAdobeOutputFormat(src, adobeOutputFormatPNG, nil)
	require.Equal(t, src, got)
}

func TestTranscodeAdobeGIFIdentity(t *testing.T) {
	src := adobeTestGIF(t)
	got, err := transcodeAdobeImage(src, adobeOutputFormatGIF, nil)
	require.NoError(t, err)
	require.Equal(t, src, got)
	require.Equal(t, adobeOutputFormatGIF, sniffAdobeImageFormat(got))
}

func TestTranscodeAdobeGIFRejectsStaticBytes(t *testing.T) {
	_, err := transcodeAdobeImage(adobeTestPNG(t, false), adobeOutputFormatGIF, nil)
	require.ErrorContains(t, err, "requested gif")
}

func TestTranscodeAdobeImageJPEGCompression(t *testing.T) {
	src := adobeTestJPEG(t, 100)
	same, err := transcodeAdobeImage(src, adobeOutputFormatJPEG, intPtr(100))
	require.NoError(t, err)
	require.Equal(t, src, same)

	sameNil, err := transcodeAdobeImage(src, adobeOutputFormatJPEG, nil)
	require.NoError(t, err)
	require.Equal(t, src, sameNil)

	recompressed, err := transcodeAdobeImage(src, adobeOutputFormatJPEG, intPtr(50))
	require.NoError(t, err)
	require.NotEqual(t, src, recompressed)
	require.Equal(t, adobeOutputFormatJPEG, sniffAdobeImageFormat(recompressed))
}

func adobeTestPNG(t *testing.T, withAlpha bool) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	alpha := uint8(255)
	if withAlpha {
		alpha = 128
	}
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(80 + x*40), B: uint8(80 + y*40), A: alpha})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func adobeTestJPEG(t *testing.T, quality int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 32), G: uint8(y * 32), B: 80, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}))
	return buf.Bytes()
}

func adobeTestGIF(t *testing.T) []byte {
	t.Helper()
	frames := []*image.Paletted{
		image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.Black, color.White}),
		image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.Black, color.RGBA{R: 255, A: 255}}),
	}
	for i, frame := range frames {
		for p := range frame.Pix {
			frame.Pix[p] = uint8((p + i) % 2)
		}
	}
	var buf bytes.Buffer
	require.NoError(t, gif.EncodeAll(&buf, &gif.GIF{Image: frames, Delay: []int{20, 20}}))
	return buf.Bytes()
}
