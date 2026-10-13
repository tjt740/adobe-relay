//go:build unit

package service

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdobeReportAttachmentContainsEntireReport(t *testing.T) {
	body := "<!doctype html><h1>完整报表</h1>" + strings.Repeat("<p>账号记录 &lt;script&gt;</p>", 300)
	m, err := buildAdobeReportMessage(&SMTPConfig{Host: "smtp.example.test", From: "from@example.test", FromName: "Sub2告警"}, "owner@example.test", "账号异常报表", body)
	require.NoError(t, err)
	parsed, err := mail.ReadMessage(bytes.NewReader(m.data))
	require.NoError(t, err)
	media, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/mixed", media)
	reader := multipart.NewReader(parsed.Body, params["boundary"])
	for i := 0; i < 2; i++ {
		part, err := reader.NextPart()
		require.NoError(t, err)
		raw, err := io.ReadAll(part)
		require.NoError(t, err)
		require.Equal(t, body, string(raw))
		if i == 1 {
			require.Equal(t, "Adobe-account-report.html", part.FileName())
		}
	}
	_, err = reader.NextPart()
	require.ErrorIs(t, err, io.EOF)
}
