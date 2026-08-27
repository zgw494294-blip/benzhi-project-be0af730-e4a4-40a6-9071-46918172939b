package caption

import "subtitle-review/internal/domain"

type CueChange struct {
	OldCueID string             `json:"oldCueID,omitempty"`
	NewCueID string             `json:"newCueID,omitempty"`
	Sequence int                `json:"sequence"`
	Type     string             `json:"type"`
	Before   *domain.CaptionCue `json:"before,omitempty"`
	After    *domain.CaptionCue `json:"after,omitempty"`
}

func Compare(parent, child []domain.CaptionCue) []CueChange {
	// Stable cue IDs anchor unchanged cues. Unmatched runs between anchors are
	// paired as modifications, with any excess classified as additions/removals.
	lcs := make([][]int, len(parent)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(child)+1)
	}
	for i := len(parent) - 1; i >= 0; i-- {
		for j := len(child) - 1; j >= 0; j-- {
			if parent[i].CueID == child[j].CueID {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	anchors := [][2]int{{-1, -1}}
	for i, j := 0, 0; i < len(parent) && j < len(child); {
		if parent[i].CueID == child[j].CueID {
			anchors = append(anchors, [2]int{i, j})
			i, j = i+1, j+1
		} else if lcs[i+1][j] >= lcs[i][j+1] {
			i++
		} else {
			j++
		}
	}
	anchors = append(anchors, [2]int{len(parent), len(child)})
	changes := make([]CueChange, 0)
	for k := 1; k < len(anchors); k++ {
		oldStart, oldEnd := anchors[k-1][0]+1, anchors[k][0]
		newStart, newEnd := anchors[k-1][1]+1, anchors[k][1]
		oldCount, newCount := oldEnd-oldStart, newEnd-newStart
		matchedOld, matchedNew := make([]bool, oldCount), make([]bool, newCount)
		for oldOffset := 0; oldOffset < oldCount; oldOffset++ {
			bestOffset, bestScore := -1, 0
			for newOffset := 0; newOffset < newCount; newOffset++ {
				if matchedNew[newOffset] {
					continue
				}
				score := cueSimilarity(parent[oldStart+oldOffset], child[newStart+newOffset])
				if score > bestScore {
					bestOffset, bestScore = newOffset, score
				}
			}
			if bestOffset < 0 && oldCount == 1 && newCount == 1 {
				bestOffset = 0
			}
			if bestOffset >= 0 {
				before, after := parent[oldStart+oldOffset], child[newStart+bestOffset]
				matchedOld[oldOffset], matchedNew[bestOffset] = true, true
				changes = append(changes, CueChange{OldCueID: before.CueID, NewCueID: after.CueID, Sequence: after.Sequence, Type: "changed", Before: &before, After: &after})
			}
		}
		for offset := 0; offset < oldCount; offset++ {
			if matchedOld[offset] {
				continue
			}
			before := parent[oldStart+offset]
			changes = append(changes, CueChange{OldCueID: before.CueID, Sequence: before.Sequence, Type: "removed", Before: &before})
		}
		for offset := 0; offset < newCount; offset++ {
			if matchedNew[offset] {
				continue
			}
			after := child[newStart+offset]
			changes = append(changes, CueChange{NewCueID: after.CueID, Sequence: after.Sequence, Type: "added", After: &after})
		}
	}
	return changes
}

func cueSimilarity(before, after domain.CaptionCue) int {
	score := 0
	if before.StartMillis < after.EndMillis && after.StartMillis < before.EndMillis {
		score += 4
	}
	if before.Text == after.Text {
		score += 3
	}
	if before.Speaker != "" && before.Speaker == after.Speaker {
		score += 2
	}
	return score
}

func CoverIssues(existing []domain.ValidationIssue, parent, child []domain.CaptionCue, childRevisionID string) []domain.ValidationIssue {
	changes := Compare(parent, child)
	changed := make(map[string]bool)
	for _, c := range changes {
		if c.OldCueID != "" {
			changed[c.OldCueID] = true
		}
	}
	result := make([]domain.ValidationIssue, len(existing))
	copy(result, existing)
	for i := range result {
		if result[i].Resolved {
			continue
		}
		if changed[result[i].CueID] {
			result[i].Resolved = true
			result[i].CoveredByRevisionID = childRevisionID
		}
	}
	return result
}
