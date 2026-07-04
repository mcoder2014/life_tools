package life_codex

import (
	"encoding/base64"
	"os"
	"testing"
)

var tinyPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
	0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
	0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xdd, 0x8d,
	0xb0, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
	0x44, 0xae, 0x42, 0x60, 0x82,
}

func TestSaveImagePayloads(t *testing.T) {
	image := ImagePayload{
		Name:        "paste.png",
		ContentType: "image/png",
		DataBase64:  base64.StdEncoding.EncodeToString(tinyPNG),
	}
	if err := ValidateImagePayloads([]ImagePayload{image}, 1, 1024); err != nil {
		t.Fatalf("validate image: %v", err)
	}
	saved, err := SaveImagePayloads(t.TempDir(), "session", "cmd", []ImagePayload{image})
	if err != nil {
		t.Fatalf("save image: %v", err)
	}
	if len(saved) != 1 || saved[0].LocalPath == "" || saved[0].DataBase64 != "" || saved[0].SHA256 == "" {
		t.Fatalf("unexpected saved metadata: %+v", saved)
	}
	if _, err := os.Stat(saved[0].LocalPath); err != nil {
		t.Fatalf("saved file missing: %v", err)
	}
}

func TestValidateImageRejectsTraversal(t *testing.T) {
	image := ImagePayload{
		Name:        "../secret.png",
		ContentType: "image/png",
		DataBase64:  base64.StdEncoding.EncodeToString(tinyPNG),
	}
	if err := ValidateImagePayloads([]ImagePayload{image}, 1, 1024); err == nil {
		t.Fatalf("expected traversal name to be rejected")
	}
}
