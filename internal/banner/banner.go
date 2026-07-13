package banner

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

const Art = `                         _ __
  ______________  ____  (_) /_
 / ___/ ___/ __ \/ __ \/ / __/
/ /__/ /  / /_/ / / / / / /_
\___/_/   \____/_/ /_/_/\__/`

type rgb struct{ r, g, b int }

var (
	gradFrom = rgb{0x49, 0x53, 0xE4}
	gradTo   = rgb{0xBB, 0x8C, 0xEE}
)

func Render() string {
	return gradient(Art)
}

func hexAt(col, denom int) string {
	if denom < 1 {
		denom = 1
	}
	t := float64(col) / float64(denom)
	lerp := func(a, b int) int { return int(float64(a) + (float64(b)-float64(a))*t + 0.5) }
	return fmt.Sprintf("#%02X%02X%02X",
		lerp(gradFrom.r, gradTo.r), lerp(gradFrom.g, gradTo.g), lerp(gradFrom.b, gradTo.b))
}

func gradient(s string) string {
	lines := strings.Split(s, "\n")
	maxW := 0
	for _, ln := range lines {
		if w := len([]rune(ln)); w > maxW {
			maxW = w
		}
	}
	denom := maxW - 1
	if denom < 1 {
		denom = 1
	}

	cache := make(map[int]lipgloss.Style, maxW)
	styleAt := func(col int) lipgloss.Style {
		if st, ok := cache[col]; ok {
			return st
		}
		st := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(hexAt(col, denom)))
		cache[col] = st
		return st
	}

	var b strings.Builder
	for li, ln := range lines {
		if li > 0 {
			b.WriteByte('\n')
		}
		runes := []rune(ln)
		for i, r := range runes {
			if r == ' ' {
				b.WriteRune(r)
				continue
			}
			b.WriteString(styleAt(i).Render(string(r)))
		}
		b.WriteString(strings.Repeat(" ", maxW-len(runes)))
	}
	return b.String()
}
