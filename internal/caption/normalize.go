package caption

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"subtitle-review/internal/domain"
)

var whitespace = regexp.MustCompile(`[ \t]+`)

func NormalizeCues(input []domain.CaptionCue) []domain.CaptionCue {
	cues := make([]domain.CaptionCue, len(input))
	copy(cues, input)
	sort.SliceStable(cues, func(i, j int) bool {
		if cues[i].StartMillis == cues[j].StartMillis {
			return cues[i].Sequence < cues[j].Sequence
		}
		return cues[i].StartMillis < cues[j].StartMillis
	})
	for i := range cues {
		cues[i].Sequence = i + 1
		cues[i].Text = normalizeText(cues[i].Text)
		cues[i].Speaker = strings.TrimSpace(cues[i].Speaker)
		cues[i].LineCount = lineCount(cues[i].Text)
		cues[i].CueID = StableCueID(cues[i])
	}
	return cues
}

func normalizeText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(whitespace.ReplaceAllString(lines[i], " "))
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n")
}

func lineCount(text string) int {
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}

func StableCueID(c domain.CaptionCue) string {
	base := fmt.Sprintf("%d|%d|%s|%s", c.StartMillis, c.EndMillis, c.Text, c.Speaker)
	return "cue_" + ShortDigest([]byte(base), 12)
}
