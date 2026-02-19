package llm

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"regexp"
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

// imagePathRe matches file paths ending with image extensions
var imagePathRe = regexp.MustCompile(`(?:~?/)?[\w./_-]+\.(?:png|jpg|jpeg|gif|webp)\b`)

// ParseImageBlocks extracts image file paths from input, returns image blocks + remaining text
func ParseImageBlocks(input string) ([]ContentBlock, string) {
	matches := imagePathRe.FindAllString(input, -1)
	if len(matches) == 0 {
		return nil, input
	}

	var blocks []ContentBlock
	remaining := input
	for _, m := range matches {
		path := expandHome(m)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		ext := strings.ToLower(filepath.Ext(m))
		mime, ok := imageExts[ext]
		if !ok {
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
		remaining = strings.Replace(remaining, m, "", 1)
	}

	remaining = strings.TrimSpace(remaining)
	return blocks, remaining
}
