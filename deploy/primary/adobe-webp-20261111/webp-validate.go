// One-shot release verification tool. Decode the complete file without changing it.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"golang.org/x/image/webp"
	"image"
	"io"
	"os"
)

func main() {
	data, err := io.ReadAll(io.LimitReader(os.Stdin, 64*1024*1024+1))
	must(err)
	if len(data) > 64*1024*1024 {
		panic("image exceeds validation limit")
	}
	cfg, err := webp.DecodeConfig(bytes.NewReader(data))
	must(err)
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 8192 || cfg.Height > 8192 {
		panic("invalid dimensions")
	}
	img, err := webp.Decode(bytes.NewReader(data))
	must(err)
	if img.Bounds() != image.Rect(0, 0, cfg.Width, cfg.Height) {
		panic("decoded dimensions disagree")
	}
	min, max := uint32(65535), uint32(0)
	for y := 0; y < cfg.Height; y++ {
		for x := 0; x < cfg.Width; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a < min {
				min = a
			}
			if a > max {
				max = a
			}
		}
	}
	h := sha256.Sum256(data)
	must(json.NewEncoder(os.Stdout).Encode(map[string]any{"format": "webp", "width": cfg.Width, "height": cfg.Height, "bytes": len(data), "sha256": hex.EncodeToString(h[:]), "fully_decoded": true, "alpha_min": min, "alpha_max": max, "has_transparency": min < 65535}))
}
func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
