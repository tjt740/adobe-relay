package service

import (
	"bytes"
	"encoding/base64"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIImagesMultipartSharesImmutableUploadsAndPreservesRelayBody(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-2.5-sunburst"))
	require.NoError(t, writer.WriteField("prompt", "memory regression"))
	require.NoError(t, writer.WriteField("unknown_provider_option", "keep this"))
	original := bytes.Repeat([]byte("source-image"), 400000)
	mask := []byte("mask-image")
	for _, field := range []string{"image[]", "mask"} {
		part, err := writer.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="` + field + `"; filename="input.png"`}, "Content-Type": {"image/png"}})
		require.NoError(t, err)
		if field == "mask" {
			_, err = part.Write(mask)
		} else {
			_, err = part.Write(original)
		}
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	raw := body.Bytes()
	saved := bytes.Clone(raw)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/images/edits", bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	parsed, err := (&OpenAIGatewayService{}).ParseOpenAIImagesRequest(c, raw)
	require.NoError(t, err)
	require.Len(t, parsed.Uploads, 1)
	require.NotNil(t, parsed.MaskUpload)
	for _, upload := range []OpenAIImagesUpload{parsed.Uploads[0], *parsed.MaskUpload} {
		offset := bytes.Index(raw, upload.Data)
		require.GreaterOrEqual(t, offset, 0)
		require.True(t, &raw[offset] == &upload.Data[0], "large uploads must not retain a second allocation")
		require.Equal(t, len(upload.Data), cap(upload.Data), "append must not overwrite multipart fields")
	}
	audit := parsed.ModerationBody()
	require.Equal(t, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(original), gjson.GetBytes(audit, "images.0.image_url").String())
	require.Equal(t, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(mask), gjson.GetBytes(audit, "images.1.image_url").String())
	forwarded, _, err := rewriteOpenAIImagesModel(raw, parsed.ContentType, "gpt-image-2")
	require.NoError(t, err)
	require.Contains(t, string(forwarded), "unknown_provider_option")
	require.Equal(t, saved, raw)
	require.Equal(t, saved, parsed.Body)
}

func TestOpenAIImagesMultipartDecodedTransferEncodingKeepsDecodedData(t *testing.T) {
	raw := []byte("--boundary\r\nContent-Disposition: form-data; name=\"image\"; filename=\"a.png\"\r\nContent-Type: image/png\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\nA=00B=FFZ\r\n--boundary--\r\n")
	req := &OpenAIImagesRequest{Endpoint: openAIImagesEditsEndpoint}
	require.NoError(t, parseOpenAIImagesMultipartRequest(raw, "multipart/form-data; boundary=boundary", req))
	require.Len(t, req.Uploads, 1)
	require.Equal(t, []byte{'A', 0, 'B', 255, 'Z'}, req.Uploads[0].Data)
	require.NotContains(t, string(raw), string(req.Uploads[0].Data))
}
