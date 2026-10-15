package imagebuffer

import (
	"encoding/base64"
	"encoding/json"
	"io"
)

// OpenAIResponse writes Base64 directly to reserved disk. It never
// creates a full Base64 string or a second full JSON image copy on the Go heap.
func OpenAIResponse(a *Arena, payload map[string]any, images [][]byte) ([]byte, error) {
	header, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	limit := int64(len(header) + 64 + len(images)*32)
	for _, img := range images {
		limit += int64(base64.StdEncoding.EncodedLen(len(img)))
	}
	return a.Build(limit, func(w io.Writer) error {
		if _, err := w.Write(header[:len(header)-1]); err != nil {
			return err
		}
		if len(payload) > 0 {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, `"data":[`); err != nil {
			return err
		}
		for i, img := range images {
			if i > 0 {
				if _, err := io.WriteString(w, ","); err != nil {
					return err
				}
			}
			if _, err := io.WriteString(w, `{"b64_json":"`); err != nil {
				return err
			}
			encoder := base64.NewEncoder(base64.StdEncoding, w)
			if _, err := encoder.Write(img); err != nil {
				return err
			}
			if err := encoder.Close(); err != nil {
				return err
			}
			if _, err := io.WriteString(w, `"}`); err != nil {
				return err
			}
		}
		_, err := io.WriteString(w, `]}`)
		return err
	})
}
