package life_codex

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var allowedImageMIMEs = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
}

func ValidateImagePayloads(images []ImagePayload, maxCount int, maxBytes int64) error {
	if len(images) == 0 {
		return nil
	}
	if maxCount <= 0 {
		maxCount = 5
	}
	if maxBytes <= 0 {
		maxBytes = 10 << 20
	}
	if len(images) > maxCount {
		return fmt.Errorf("too many images: %d > %d", len(images), maxCount)
	}
	for _, image := range images {
		if image.DataBase64 == "" && image.LocalPath == "" {
			return fmt.Errorf("image %q has no data", image.Name)
		}
		if image.DataBase64 != "" {
			decoded, err := base64.StdEncoding.DecodeString(image.DataBase64)
			if err != nil {
				return fmt.Errorf("decode image %q: %w", image.Name, err)
			}
			if int64(len(decoded)) > maxBytes {
				return fmt.Errorf("image %q is too large", image.Name)
			}
			contentType := image.ContentType
			if contentType == "" {
				contentType = http.DetectContentType(decoded)
			}
			if _, ok := allowedImageMIMEs[contentType]; !ok {
				return fmt.Errorf("unsupported image content type: %s", contentType)
			}
		}
		if strings.Contains(image.Name, "/") || strings.Contains(image.Name, "\\") || strings.Contains(image.Name, "..") {
			return fmt.Errorf("unsafe image name: %s", image.Name)
		}
	}
	return nil
}

func SaveImagePayloads(root string, sessionID string, commandID string, images []ImagePayload) ([]ImagePayload, error) {
	if len(images) == 0 {
		return nil, nil
	}
	if root == "" {
		return nil, fmt.Errorf("attachment dir is required")
	}
	targetDir := filepath.Join(root, safePathPart(sessionID), safePathPart(commandID))
	if err := os.MkdirAll(targetDir, 0700); err != nil {
		return nil, err
	}
	out := make([]ImagePayload, 0, len(images))
	for i, image := range images {
		if image.DataBase64 == "" {
			out = append(out, image)
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(image.DataBase64)
		if err != nil {
			return nil, err
		}
		contentType := image.ContentType
		if contentType == "" {
			contentType = http.DetectContentType(decoded)
		}
		ext, ok := allowedImageMIMEs[contentType]
		if !ok {
			return nil, fmt.Errorf("unsupported image content type: %s", contentType)
		}
		sum := sha256.Sum256(decoded)
		sha := hex.EncodeToString(sum[:])
		name := safePathPart(image.Name)
		if name == "" {
			name = fmt.Sprintf("image-%d%s", i+1, ext)
		}
		if filepath.Ext(name) == "" {
			name += ext
		}
		path := filepath.Join(targetDir, name)
		if !PathInAllowedRoots(path, []string{targetDir}) {
			return nil, fmt.Errorf("unsafe attachment path")
		}
		if err := os.WriteFile(path, decoded, 0600); err != nil {
			return nil, err
		}
		image.ContentType = contentType
		image.Size = int64(len(decoded))
		image.SHA256 = sha
		image.LocalPath = path
		image.DataBase64 = ""
		out = append(out, image)
	}
	return out, nil
}

func ImageMetadata(images []ImagePayload) []ImagePayload {
	out := make([]ImagePayload, 0, len(images))
	for _, image := range images {
		image.DataBase64 = ""
		out = append(out, image)
	}
	return out
}

func safePathPart(value string) string {
	value = filepath.Base(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, string(filepath.Separator), "_")
	if value == "." || value == "/" || value == "\\" {
		return ""
	}
	return value
}
