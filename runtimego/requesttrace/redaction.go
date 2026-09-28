package requesttrace

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/url"
	"strings"
	"unicode/utf8"
)

const imageRedactionReason = "image_input"

type BodySnapshot struct {
	Body            []byte
	ContentType     string
	Redacted        bool
	RedactionReason string
	OriginalSize    int64
}

func SanitizeBody(body []byte, contentType string, forceImageRequest bool) BodySnapshot {
	snapshot := BodySnapshot{Body: body, ContentType: contentType}
	if len(body) == 0 {
		return snapshot
	}

	mediaType, _, _ := mime.ParseMediaType(contentType)
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	switch {
	case mediaType == "application/x-www-form-urlencoded":
		redacted, changed := redactFormCredentials(body)
		if changed {
			return BodySnapshot{Body: redacted, ContentType: contentType, Redacted: true, RedactionReason: "credentials", OriginalSize: int64(len(body))}
		}
		return snapshot
	case strings.HasPrefix(mediaType, "image/"):
		return redactedBody(nil, len(body))
	case strings.HasPrefix(mediaType, "multipart/"):
		redacted, changed, err := redactMultipartBody(body, contentType, forceImageRequest)
		if err != nil {
			if forceImageRequest || bodyMayContainImageInput(body) {
				return redactedBody(nil, len(body))
			}
			return snapshot
		}
		if changed {
			return redactedBody(redacted, len(body))
		}
		return snapshot
	case isJSONMediaType(mediaType) || json.Valid(body):
		redacted, changed, credentialsOnly, err := redactJSONBody(body)
		if err != nil {
			if forceImageRequest || bodyMayContainImageInput(body) {
				return redactedBody(nil, len(body))
			}
			return snapshot
		}
		if changed {
			result := redactedBody(redacted, len(body))
			if credentialsOnly {
				result.RedactionReason = "credentials"
			}
			return result
		}
		return snapshot
	case forceImageRequest:
		return redactedBody(nil, len(body))
	default:
		return snapshot
	}
}

// Shared HTTP clients can also perform token refreshes. Never retain those
// credentials in request or response bodies, even when a later step fails.
func redactFormCredentials(body []byte) ([]byte, bool) {
	fields, err := url.ParseQuery(string(body))
	if err != nil {
		return body, false
	}
	changed := false
	for key := range fields {
		if isCredentialField(key) {
			fields.Set(key, "[REDACTED]")
			changed = true
		}
	}
	if changed {
		return []byte(fields.Encode()), true
	}
	return body, false
}

func redactCredentialValue(value any) bool {
	changed := false
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			if isCredentialField(key) {
				value[key] = "[REDACTED]"
				changed = true
			} else if redactCredentialValue(child) {
				changed = true
			}
		}
	case []any:
		for _, child := range value {
			if redactCredentialValue(child) {
				changed = true
			}
		}
	}
	return changed
}

func isCredentialField(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "access_token", "refresh_token", "id_token", "api_key", "client_secret", "authorization", "password", "cookie":
		return true
	}
	return false
}

func redactedBody(body []byte, originalSize int) BodySnapshot {
	return BodySnapshot{
		Body:            body,
		ContentType:     "application/json",
		Redacted:        true,
		RedactionReason: imageRedactionReason,
		OriginalSize:    int64(originalSize),
	}
}

func redactJSONBody(body []byte) ([]byte, bool, bool, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, false, false, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, false, false, errors.New("request body contains trailing JSON data")
	}
	credentials := redactCredentialValue(value)
	redacted, keep, changed := redactJSONValue(value)
	if !keep {
		redacted = map[string]any{}
		changed = true
	}
	if !changed && !credentials {
		return body, false, false, nil
	}
	encoded, err := json.Marshal(redacted)
	if err != nil {
		return nil, false, false, err
	}
	return encoded, true, credentials && !changed, nil
}

func redactJSONValue(value any) (any, bool, bool) {
	switch current := value.(type) {
	case []any:
		out := make([]any, 0, len(current))
		changed := false
		for _, item := range current {
			redacted, keep, itemChanged := redactJSONValue(item)
			changed = changed || itemChanged
			if !keep {
				changed = true
				continue
			}
			out = append(out, redacted)
		}
		return out, true, changed
	case map[string]any:
		if isImageContentBlock(current) {
			return nil, false, true
		}
		out := make(map[string]any, len(current))
		changed := false
		for key, item := range current {
			if isImageField(key) {
				changed = true
				continue
			}
			redacted, keep, itemChanged := redactJSONValue(item)
			changed = changed || itemChanged
			if !keep {
				changed = true
				continue
			}
			out[key] = redacted
		}
		return out, true, changed
	case string:
		if isInlineImage(current) {
			return nil, false, true
		}
	}
	return value, true, false
}

func isImageContentBlock(value map[string]any) bool {
	itemType, _ := value["type"].(string)
	switch strings.ToLower(strings.TrimSpace(itemType)) {
	case "image", "image_url", "input_image", "input_image_url", "image_file", "input_image_file", "computer_screenshot", "screenshot":
		return true
	}
	mediaType, _ := value["media_type"].(string)
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(mediaType)), "image/") {
		return true
	}
	mimeType, _ := value["mime_type"].(string)
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(mimeType)), "image/")
}

func isImageField(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "image", "images", "mask", "masks",
		"image_url", "image_urls", "input_image", "input_images",
		"input_image_url", "input_image_urls", "image_data", "image_base64",
		"input_image_data", "reference_image", "reference_images",
		"source_image", "source_images", "init_image", "init_images",
		"b64_json", "partial_image_b64", "partial_image", "image_b64":
		return true
	default:
		return false
	}
}

func isInlineImage(value string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "data:image/")
}

func bodyMayContainImageInput(body []byte) bool {
	for _, marker := range [][]byte{
		[]byte("data:image/"),
		[]byte(`"image"`),
		[]byte(`"images"`),
		[]byte(`"mask"`),
		[]byte(`"image_url"`),
		[]byte(`"input_image"`),
		[]byte(`"b64_json"`),
		[]byte(`"partial_image_b64"`),
		[]byte(`"partial_image"`),
		[]byte(`"image_b64"`),
		[]byte("image/"),
		[]byte(".png\""),
		[]byte(".jpg\""),
		[]byte(".jpeg\""),
		[]byte(".webp\""),
		[]byte(".gif\""),
	} {
		if bytes.Contains(body, marker) {
			return true
		}
	}
	return false
}

func redactMultipartBody(body []byte, contentType string, canonicalize bool) ([]byte, bool, error) {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, false, err
	}
	boundary := params["boundary"]
	if boundary == "" {
		return nil, false, errors.New("multipart content type is missing boundary")
	}

	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	fields := make(map[string][]string)
	changed := false
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, false, err
		}
		data, readErr := io.ReadAll(part)
		name := part.FormName()
		filename := part.FileName()
		partContentType := strings.ToLower(strings.TrimSpace(part.Header.Get("Content-Type")))
		_ = part.Close()
		if readErr != nil {
			return nil, false, readErr
		}
		drop := filename != "" || isImageField(name) ||
			strings.HasPrefix(partContentType, "image/") || isInlineImage(string(data))
		if !utf8.Valid(data) || strings.EqualFold(partContentType, "application/octet-stream") {
			drop = true
		}
		if drop {
			changed = true
			continue
		}
		fields[name] = append(fields[name], string(data))
	}
	if !changed && !canonicalize {
		return body, false, nil
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return nil, false, err
	}
	return encoded, true, nil
}

func isJSONMediaType(mediaType string) bool {
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func IsImagePath(path string) bool {
	path = strings.ToLower(strings.TrimSpace(path))
	if index := strings.IndexByte(path, '?'); index >= 0 {
		path = path[:index]
	}
	return strings.HasSuffix(path, "/images/generations") || strings.HasSuffix(path, "/images/edits")
}

func IsImageURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err == nil && parsed != nil {
		return IsImagePath(parsed.Path)
	}
	return IsImagePath(raw)
}
