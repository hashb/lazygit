package utils

import (
	"regexp"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/gui/style"
)

// RenderMarkdown takes a markdown string and returns a terminal-styled string
// suitable for display in lazygit's main view. It handles the most common
// markdown elements found in git commit messages.
func RenderMarkdown(markdown string) string {
	lines := strings.Split(markdown, "\n")
	var result []string
	inCodeBlock := false
	codeBlockFence := ""

	// tableRows accumulates pipe-delimited lines for a pending table.
	var tableRows []string

	flushTable := func() {
		if len(tableRows) > 0 {
			result = append(result, renderTable(tableRows)...)
			tableRows = nil
		}
	}

	for _, line := range lines {
		// Detect fenced code block boundaries (``` or ~~~)
		if !inCodeBlock {
			if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
				flushTable()
				inCodeBlock = true
				codeBlockFence = line[:3]
				result = append(result, style.FgCyan.Sprint(line))
				continue
			}
		} else {
			if strings.HasPrefix(line, codeBlockFence) {
				inCodeBlock = false
				codeBlockFence = ""
				result = append(result, style.FgCyan.Sprint(line))
				continue
			}
			// Inside a code block: highlight and skip inline processing
			result = append(result, style.FgYellow.Sprint(line))
			continue
		}

		// GFM tables: lines that start and end with '|', or contain '|'
		if isTableRow(line) {
			tableRows = append(tableRows, line)
			continue
		}

		// Non-table line: flush any accumulated table first
		flushTable()

		// ATX headers (#, ##, ###, etc.)
		if strings.HasPrefix(line, "#") {
			level := 0
			for level < len(line) && line[level] == '#' {
				level++
			}
			if level < len(line) && line[level] == ' ' {
				text := strings.TrimSpace(line[level+1:])
				switch level {
				case 1:
					result = append(result, style.FgMagenta.SetBold().Sprint(text))
				case 2:
					result = append(result, style.FgBlue.SetBold().Sprint(text))
				default:
					result = append(result, style.FgCyan.SetBold().Sprint(text))
				}
				continue
			}
		}

		// Horizontal rules: a line of only dashes, asterisks, or underscores (3+)
		if isHorizontalRule(line) {
			result = append(result, style.FgBlackLighter.Sprint(strings.Repeat("─", 40)))
			continue
		}

		// Blockquotes
		if strings.HasPrefix(line, "> ") {
			text := renderInlineMarkdown(line[2:])
			result = append(result, style.FgBlackLighter.Sprint("▌ ")+text)
			continue
		}

		// Unordered lists: lines starting with "- ", "* ", or "+ " (possibly indented)
		stripped := strings.TrimLeft(line, " \t")
		indent := len(line) - len(stripped)
		if len(stripped) >= 2 && (stripped[0] == '-' || stripped[0] == '*' || stripped[0] == '+') && stripped[1] == ' ' {
			prefix := line[:indent]
			text := renderInlineMarkdown(stripped[2:])
			result = append(result, prefix+style.FgYellow.Sprint("• ")+text)
			continue
		}

		// Numbered lists (1., 2., etc.)
		if m := numberedListRe.FindStringSubmatchIndex(line); m != nil {
			num := line[m[2]:m[3]]
			rest := renderInlineMarkdown(line[m[4]:m[5]])
			result = append(result, style.FgYellow.Sprint(num+". ")+rest)
			continue
		}

		// Regular paragraph text with inline markdown
		result = append(result, renderInlineMarkdown(line))
	}

	// Flush any trailing table
	flushTable()

	return strings.Join(result, "\n")
}

// isTableRow returns true if the line looks like a GFM table row (contains '|').
func isTableRow(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.Contains(trimmed, "|")
}

// isSeparatorRow returns true if the line is a GFM table separator (e.g. |---|---|).
func isSeparatorRow(line string) bool {
	for _, ch := range strings.TrimSpace(line) {
		if ch != '|' && ch != '-' && ch != ':' && ch != ' ' {
			return false
		}
	}
	return strings.Contains(line, "-")
}

// parseTableCells splits a table row into cell strings, trimming surrounding pipes and whitespace.
func parseTableCells(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	parts := strings.Split(trimmed, "|")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

// renderTable renders a slice of raw table lines into styled terminal lines.
// It finds the separator row to identify headers, computes column widths, and
// produces an aligned, bordered table.
func renderTable(rows []string) []string {
	if len(rows) == 0 {
		return nil
	}

	// Find the separator row index (e.g. |---|---|)
	separatorIdx := -1
	for i, row := range rows {
		if isSeparatorRow(row) {
			separatorIdx = i
			break
		}
	}

	// Parse all rows into cells
	parsed := make([][]string, len(rows))
	for i, row := range rows {
		parsed[i] = parseTableCells(row)
	}

	// Determine column count (max across all non-separator rows)
	numCols := 0
	for i, cells := range parsed {
		if i == separatorIdx {
			continue
		}
		if len(cells) > numCols {
			numCols = len(cells)
		}
	}
	if numCols == 0 {
		return nil
	}

	// Compute column widths from non-separator rows
	colWidths := make([]int, numCols)
	for i, cells := range parsed {
		if i == separatorIdx {
			continue
		}
		for j, cell := range cells {
			if j < numCols && len(cell) > colWidths[j] {
				colWidths[j] = len(cell)
			}
		}
	}

	divider := buildDivider(colWidths)
	var out []string

	for i, cells := range parsed {
		// Replace the separator row with a styled divider
		if i == separatorIdx {
			out = append(out, style.FgBlackLighter.Sprint(divider))
			continue
		}

		isHeader := separatorIdx > 0 && i < separatorIdx

		var sb strings.Builder
		sb.WriteString(style.FgBlackLighter.Sprint("│"))
		for j := 0; j < numCols; j++ {
			cell := ""
			if j < len(cells) {
				cell = cells[j]
			}
			padded := cell + strings.Repeat(" ", colWidths[j]-len(cell))
			if isHeader {
				sb.WriteString(" " + style.AttrBold.Sprint(padded) + " ")
			} else {
				sb.WriteString(" " + renderInlineMarkdown(padded) + " ")
			}
			sb.WriteString(style.FgBlackLighter.Sprint("│"))
		}
		out = append(out, sb.String())

		// If there's no explicit separator and this is the first row with more rows following,
		// add a divider after row 0 to visually separate the header.
		if separatorIdx < 0 && i == 0 && len(parsed) > 1 {
			out = append(out, style.FgBlackLighter.Sprint(divider))
		}
	}

	return out
}

// buildDivider builds a horizontal divider like ├───┼───┤ for the given column widths.
func buildDivider(colWidths []int) string {
	var sb strings.Builder
	sb.WriteString("├")
	for i, w := range colWidths {
		sb.WriteString(strings.Repeat("─", w+2))
		if i < len(colWidths)-1 {
			sb.WriteString("┼")
		}
	}
	sb.WriteString("┤")
	return sb.String()
}

var numberedListRe = regexp.MustCompile(`^(\d+)\.\s+(.+)$`)

func isHorizontalRule(line string) bool {
	if len(line) < 3 {
		return false
	}
	ch := line[0]
	if ch != '-' && ch != '*' && ch != '_' {
		return false
	}
	for i := 0; i < len(line); i++ {
		if line[i] != ch && line[i] != ' ' {
			return false
		}
	}
	count := strings.Count(line, string(ch))
	return count >= 3
}

// renderInlineMarkdown applies inline markdown: code spans, bold, italic.
// Code spans are processed first to protect their content from further styling.
func renderInlineMarkdown(text string) string {
	// Split on backtick code spans, process non-code parts for bold/italic
	parts := backtickRe.Split(text, -1)
	spans := backtickRe.FindAllString(text, -1)

	var sb strings.Builder
	for i, part := range parts {
		sb.WriteString(applyBoldItalic(part))
		if i < len(spans) {
			// spans[i] is the full match including backticks, e.g. "`code`"
			inner := spans[i][1 : len(spans[i])-1]
			sb.WriteString(style.FgYellow.Sprint("`" + inner + "`"))
		}
	}
	return sb.String()
}

// backtickRe matches inline code spans delimited by single backticks.
var backtickRe = regexp.MustCompile("`[^`]+`")

// applyBoldItalic processes **bold**, __bold__, *italic*, and _italic_ markers.
func applyBoldItalic(text string) string {
	// Bold+italic must come before bold and italic
	text = boldItalicStarRe.ReplaceAllStringFunc(text, func(match string) string {
		inner := boldItalicStarRe.FindStringSubmatch(match)[1]
		return style.FgWhite.SetBold().Sprint(inner)
	})
	text = boldItalicUnderRe.ReplaceAllStringFunc(text, func(match string) string {
		inner := boldItalicUnderRe.FindStringSubmatch(match)[1]
		return style.FgWhite.SetBold().Sprint(inner)
	})

	// Bold
	text = boldStarRe.ReplaceAllStringFunc(text, func(match string) string {
		inner := boldStarRe.FindStringSubmatch(match)[1]
		return style.AttrBold.Sprint(inner)
	})
	text = boldUnderRe.ReplaceAllStringFunc(text, func(match string) string {
		inner := boldUnderRe.FindStringSubmatch(match)[1]
		return style.AttrBold.Sprint(inner)
	})

	// Italic (use underline as a terminal-friendly substitute)
	text = italicStarRe.ReplaceAllStringFunc(text, func(match string) string {
		inner := italicStarRe.FindStringSubmatch(match)[1]
		return style.AttrUnderline.Sprint(inner)
	})
	text = italicUnderRe.ReplaceAllStringFunc(text, func(match string) string {
		inner := italicUnderRe.FindStringSubmatch(match)[1]
		return style.AttrUnderline.Sprint(inner)
	})

	return text
}

var (
	boldItalicStarRe  = regexp.MustCompile(`\*\*\*(.+?)\*\*\*`)
	boldItalicUnderRe = regexp.MustCompile(`___(.+?)___`)
	boldStarRe        = regexp.MustCompile(`\*\*(.+?)\*\*`)
	boldUnderRe       = regexp.MustCompile(`__(.+?)__`)
	italicStarRe      = regexp.MustCompile(`\*([^*\s][^*]*[^*\s]|[^*\s])\*`)
	italicUnderRe     = regexp.MustCompile(`_([^_\s][^_]*[^_\s]|[^_\s])_`)
)
