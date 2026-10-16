package server

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
)

const MaxImageBytes = 512 * 1024
const MaxImages = 4
const MaxImageDimension = 1024
const MaxChatBodyBytes = 4 * 1024 * 1024

// Only bounded inline still images are accepted. Remote URLs and file paths are
// intentionally excluded so requests cannot trigger backend network/file reads.
func ValidateImageURL(value string) error {
	header, encoded, ok := strings.Cut(value, ",")
	if !ok || (header != "data:image/png;base64" && header != "data:image/jpeg;base64") {
		return fmt.Errorf("images must be inline PNG/JPEG base64 data URLs")
	}
	if len(encoded) > base64.StdEncoding.EncodedLen(MaxImageBytes) {
		return fmt.Errorf("image exceeds 512 KiB")
	}
	data, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(data) > MaxImageBytes {
		return fmt.Errorf("invalid or oversized image base64")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid image: %w", err)
	}
	if "data:image/"+format+";base64" != header {
		return fmt.Errorf("image MIME type does not match contents")
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > MaxImageDimension || cfg.Height > MaxImageDimension {
		return fmt.Errorf("image dimensions must be between 1 and 1024 pixels per side")
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return fmt.Errorf("invalid image: %w", err)
	}
	return nil
}

func validateChatContent(messages []ChatMessage, vision bool) error {
	if vision && (len(messages) == 0 || len(messages) > 256) {
		return fmt.Errorf("chat requires between 1 and 256 messages")
	}
	count := 0
	for _, message := range messages {
		switch content := message.Content.(type) {
		case nil, string:
		case []any:
			for _, raw := range content {
				part, ok := raw.(map[string]any)
				if !ok {
					return fmt.Errorf("invalid content part")
				}
				switch part["type"] {
				case "text":
					if _, ok := part["text"].(string); !ok {
						return fmt.Errorf("text part requires text")
					}
				case "image_url":
					if !vision {
						return fmt.Errorf("selected model does not support image input")
					}
					if message.Role != "user" {
						return fmt.Errorf("images are only supported in user messages")
					}
					count++
					if count > MaxImages {
						return fmt.Errorf("at most four images are supported across conversation history")
					}
					imageURL, ok := part["image_url"].(map[string]any)
					if !ok {
						return fmt.Errorf("invalid image_url part")
					}
					value, ok := imageURL["url"].(string)
					if !ok {
						return fmt.Errorf("image_url requires a URL")
					}
					if err := ValidateImageURL(value); err != nil {
						return err
					}
				default:
					return fmt.Errorf("unsupported content modality %v; use text or image_url", part["type"])
				}
			}
		default:
			return fmt.Errorf("message content must be text or an array of content parts")
		}
	}
	return nil
}

// Adapter endpoints currently implement text/tool conversion only. Reject media
// before conversion, which would otherwise silently strip these blocks.
func containsMedia(value any) bool {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			if containsMedia(item) {
				return true
			}
		}
	case map[string]any:
		if kind, ok := v["type"].(string); ok {
			for _, media := range []string{"image", "image_url", "input_image", "input_audio", "audio", "video", "input_video", "file", "input_file", "document"} {
				if kind == media {
					return true
				}
			}
		}
		for _, item := range v {
			if containsMedia(item) {
				return true
			}
		}
	}
	return false
}

func hasMessageMedia(messages []ChatMessage) bool {
	for _, message := range messages {
		if containsMedia(message.Content) {
			return true
		}
	}
	return false
}
