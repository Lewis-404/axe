package llm

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
)

var imageExts = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// ParseImageBlocks scans input for image file paths, returns text + image content blocks
func ParseImageBlocks(input string) ([]ContentBlock, string) {
	words := strings.Fields(input)
	var blocks []ContentBlock
	var textParts []string

	for _, w := range words {
		ext := strings.ToLower(filepath.Ext(w))
		mime, ok := imageExts[ext]
		if !ok {
			textParts = append(textParts, w)
			continue
		}
		path := expandHome(w)
		data, err := os.ReadFile(path)
		if err != nil {
			textParts = append(textParts, w)
			continue
		}
		blocks = append(blocks, ContentBlock{
			Type: "image",
			Source: &ImageSource{
				Type:      "base64",
				MediaType: mime,
				Data:      base64.StdEncoding.EncodeToString(data),
			},
		})
	}

	text := strings.Join(textParts, " ")
	return blocks, text
}
