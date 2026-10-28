package adobe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// PublicErrorDetails contains gateway-owned identifiers, never raw provider text.
type PublicErrorDetails struct {
	Code           string `json:"code"`
	Param          string `json:"param,omitempty"`
	Stage          string `json:"stage"`
	Source         string `json:"source"`
	UpstreamStatus int    `json:"upstream_status,omitempty"`
}

func NewParameterError(message, code, param string) *RequestError {
	return &RequestError{Message: message, Code: code, Param: param, Source: "request"}
}

// Keep response/protocol failures terminal: reporting 502 must not replay a POST.
func NewResponseError(message, code, userMessage string) *RequestError {
	return &RequestError{Message: message, Code: code, UserMessage: userMessage,
		Source: "upstream", ClientStatusCode: 502}
}

func NewInternalError(message, code string) *RequestError {
	return &RequestError{Message: message, Code: code, UserMessage: "The image gateway could not process this request.",
		Source: "gateway", ClientStatusCode: 500}
}

// ValidationDetails classifies only local parser messages. Unknown failures retain
// a generic code; no provider payload or client-supplied field name is reflected.
func ValidationDetails(message string) PublicErrorDetails {
	d := PublicErrorDetails{Code: "invalid_request", Stage: "validation", Source: "request"}
	message = strings.ToLower(message)
	switch {
	case strings.Contains(message, "request body is empty"):
		d.Code = "empty_request_body"
	case strings.Contains(message, "failed to parse request body"):
		d.Code = "invalid_json"
	case strings.Contains(message, "failed to read request body"):
		d.Code = "request_body_read_failed"
	case strings.Contains(message, "multipart"):
		d.Code = "invalid_multipart"
	case strings.Contains(message, "request body"):
		d.Code = "invalid_request_body"
	case strings.Contains(message, "model"):
		d.Code, d.Param = "unsupported_image_model", "model"
	default:
		for _, param := range []string{"output_compression", "partial_images", "response_format", "output_format", "input_fidelity", "prompt", "size", "quality", "background", "stream", "n", "images", "mask"} {
			if strings.HasPrefix(message, "invalid "+param+" ") || strings.HasPrefix(message, param+" ") || strings.HasPrefix(message, param+"[") || strings.HasPrefix(message, param+".") {
				d.Code, d.Param = "invalid_"+param, param
				break
			}
		}
	}
	return d
}

func DescribeError(err error) PublicErrorDetails {
	d := PublicErrorDetails{Code: "upstream_error", Stage: "unknown", Source: "upstream"}
	var r *RequestError
	if errors.As(err, &r) {
		d.UpstreamStatus, d.Code, d.Param = r.StatusCode, r.Code, r.Param
		if r.Source != "" {
			d.Source = r.Source
		} else if r.StatusCode == 0 && r.ErrorType == "" {
			d.Source, d.Stage = "request", "validation"
		}
		if d.Source == "request" {
			d.Stage = "validation"
		} else if d.Source == "gateway" {
			d.Stage = "prepare"
		}
		if d.Code == "" {
			d.Code = "upstream_request_rejected"
			if d.Source == "request" {
				d.Code = "invalid_request"
			}
		}
	}
	var auth *AuthError
	var quota *QuotaExhaustedError
	var entitled *NotEntitledError
	var content *ContentRejectedError
	var temporary *UpstreamTemporaryError
	switch {
	case errors.As(err, &auth):
		d.Code = "adobe_authentication_failed"
	case errors.As(err, &quota):
		d.Code = "adobe_quota_exhausted"
	case errors.As(err, &entitled):
		d.Code = "adobe_not_entitled"
	case errors.As(err, &content):
		d.Code = "adobe_content_rejected"
	case errors.As(err, &temporary):
		d.Code = "upstream_unavailable"
		if temporary.StatusCode == 429 {
			d.Code = "upstream_rate_limited"
		} else if temporary.ErrorType == ErrorTypeTimeout {
			d.Code = "upstream_timeout"
		} else if temporary.ErrorType == ErrorTypeProxy {
			d.Code = "upstream_proxy_failed"
		} else if temporary.ErrorType == ErrorTypeConnection || temporary.ErrorType == ErrorTypeNetwork {
			d.Code = "upstream_connection_failed"
		}
	case errors.Is(err, context.DeadlineExceeded):
		d.Code = "upstream_timeout"
	case errors.Is(err, context.Canceled):
		d.Code = "request_canceled"
	}
	var op *OperationError
	if errors.As(err, &op) {
		switch op.Stage {
		case "prepare", "validation", "input_fetch", "upload", "submit", "poll", "download":
			d.Stage = op.Stage
		}
		if op.HTTPStatus > 0 {
			d.UpstreamStatus = op.HTTPStatus
		}
	}
	return d
}

var invalidImageSize = regexp.MustCompile(`(?i)invalid image size ([0-9]{1,6})x([0-9]{1,6}): both edges must be non-zero and multiples of 16`)

func publicHTTPError(status int, body, diagnostic string) *RequestError {
	r := &RequestError{Message: diagnostic, StatusCode: status, ErrorType: ErrorTypeStatus,
		Source: "upstream", Code: "upstream_request_rejected",
		UserMessage: fmt.Sprintf("Adobe rejected the request (HTTP %d)", status)}
	var payload struct {
		Code    string `json:"error_code"`
		Message string `json:"message"`
		Param   string `json:"param"`
	}
	_ = json.Unmarshal([]byte(body), &payload)
	// Only a proven validation rejection can expose extracted numeric dimensions.
	if status == 400 || status == 422 {
		if match := invalidImageSize.FindStringSubmatch(payload.Message); match != nil {
			r.Code, r.Param = "image_size_not_multiple_of_16", "size"
			r.UserMessage = fmt.Sprintf("Invalid image size %sx%s: width and height must be positive multiples of 16.", match[1], match[2])
		} else {
			switch strings.ToLower(payload.Code) {
			case "invalid_size", "invalid_image_size":
				r.Code, r.Param, r.UserMessage = "invalid_image_size", "size", "Adobe rejected the image size; use dimensions supported by the selected model."
			case "invalid_prompt", "prompt_too_long":
				r.Code, r.Param, r.UserMessage = "invalid_prompt", "prompt", "Adobe rejected the prompt format or length."
			case "invalid_quality":
				r.Code, r.Param, r.UserMessage = "invalid_image_quality", "quality", "Adobe rejected the requested image quality."
			case "invalid_background", "unsupported_background":
				r.Code, r.Param, r.UserMessage = "invalid_image_background", "background", "Adobe rejected the requested image background."
			case "invalid_output_format", "unsupported_output_format":
				r.Code, r.Param, r.UserMessage = "unsupported_output_format", "output_format", "Adobe rejected the requested output image format."
			case "invalid_n", "n_out_of_range":
				r.Code, r.Param, r.UserMessage = "invalid_n", "n", "Adobe rejected the requested image count."
			case "invalid_image", "invalid_reference_image":
				r.Code, r.Param, r.UserMessage = "invalid_input_image", "image", "Adobe rejected an input image; check its format and contents."
			case "invalid_parameter", "bad_request", "validation_error":
				switch payload.Param {
				case "size", "prompt", "quality", "background", "output_format", "n", "image", "images", "mask", "input_fidelity", "output_compression":
					r.Code, r.Param = "invalid_image_parameter", payload.Param
					r.UserMessage = fmt.Sprintf("Adobe rejected the %s parameter.", payload.Param)
				}
			}
		}
	} else {
		switch status {
		case 413:
			r.Code, r.Param, r.UserMessage = "input_image_too_large", "image", "Adobe rejected the upload because it exceeds the upstream size limit."
		case 415:
			r.Code, r.Param, r.UserMessage = "unsupported_image_format", "image", "Adobe does not support the uploaded image format."
		default:
			r.ClientStatusCode, r.Code = 502, "upstream_request_failed"
			r.UserMessage = fmt.Sprintf("Adobe could not complete the request (upstream HTTP %d).", status)
			if status == 424 {
				r.Code, r.UserMessage = "upstream_dependency_failed", "Adobe could not complete the request because an upstream dependency failed."
			} else if status == 404 || status == 410 {
				r.Code, r.UserMessage = "upstream_resource_unavailable", "The upstream endpoint, task or image resource is unavailable."
			}
		}
	}
	return r
}
