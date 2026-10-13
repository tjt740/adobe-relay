package service

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"mime/quotedprintable"
	"net/textproto"
)

// Include the same complete, self-contained report as an HTML attachment so
// recipients can open all rows even when their mail client collapses long mail.
func buildAdobeReportMessage(config *SMTPConfig, to, subject, body string) (smtpMessage, error) {
	message, err := buildSMTPMessage(config, to, subject, body)
	if err != nil {
		return smtpMessage{}, err
	}
	var mimeBody bytes.Buffer
	w := multipart.NewWriter(&mimeBody)
	for _, disposition := range []string{"inline", `attachment; filename="Adobe-account-report.html"`} {
		part, err := w.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {"text/html; charset=UTF-8"},
			"Content-Transfer-Encoding": {"quoted-printable"},
			"Content-Disposition":       {disposition},
		})
		if err != nil {
			return smtpMessage{}, err
		}
		encoded := quotedprintable.NewWriter(part)
		if _, err = encoded.Write([]byte(body)); err != nil {
			return smtpMessage{}, err
		}
		if err = encoded.Close(); err != nil {
			return smtpMessage{}, err
		}
	}
	if err = w.Close(); err != nil {
		return smtpMessage{}, err
	}
	end := bytes.Index(message.data, []byte("MIME-Version:"))
	if end < 0 {
		return smtpMessage{}, fmt.Errorf("missing MIME headers")
	}
	header := fmt.Sprintf("MIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=%q\r\n\r\n", w.Boundary())
	message.data = append(append(message.data[:end:end], []byte(header)...), mimeBody.Bytes()...)
	return message, nil
}

func (s *EmailService) sendAdobeReport(config *SMTPConfig, to, subject, body string) error {
	message, err := buildAdobeReportMessage(config, to, subject, body)
	if err != nil {
		return err
	}
	return s.sendSMTPMessage(config, message)
}
