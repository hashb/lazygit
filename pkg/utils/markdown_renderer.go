package utils

import (
	"strings"

	markdown "github.com/MichaelMure/go-term-markdown"
)

// RenderMarkdown takes a markdown string and returns a terminal-styled string
// suitable for display in lazygit's main view.
func RenderMarkdown(source string) string {
	result := markdown.Render(source, 80, 0)
	return strings.TrimSpace(string(result))
}
