package domain

import (
	"fmt"
	"strings"
	"time"
)

type ProjectRules struct {
	DurationMillis    int64   `json:"durationMillis"`
	Language          string  `json:"language"`
	FrameRate         float64 `json:"frameRate"`
	MaxCharsPerSecond float64 `json:"maxCharsPerSecond"`
	DeliveryStandard  string  `json:"deliveryStandard"`
}

func (r ProjectRules) Validate() error {
	if r.DurationMillis <= 0 {
		return FieldError("durationMillis", "节目时长必须大于零")
	}
	if strings.TrimSpace(r.Language) == "" {
		return FieldError("language", "语言不能为空")
	}
	if r.FrameRate < 1 || r.FrameRate > 240 {
		return FieldError("frameRate", "帧率必须在 1 到 240 之间")
	}
	if r.MaxCharsPerSecond <= 0 || r.MaxCharsPerSecond > 100 {
		return FieldError("maxCharsPerSecond", "阅读速度阈值必须在 0 到 100 之间")
	}
	if strings.TrimSpace(r.DeliveryStandard) == "" {
		return FieldError("deliveryStandard", "交付规范不能为空")
	}
	return nil
}

type CaptionProject struct {
	ID               string        `json:"id"`
	Title            string        `json:"title"`
	Rules            ProjectRules  `json:"rules"`
	Status           ProjectStatus `json:"status"`
	ActiveRevisionID string        `json:"activeRevisionID,omitempty"`
	Version          int64         `json:"version"`
	CreatedAt        time.Time     `json:"createdAt"`
	UpdatedAt        time.Time     `json:"updatedAt"`
}

func NewProject(id, title string, rules ProjectRules, now time.Time) (*CaptionProject, error) {
	if strings.TrimSpace(id) == "" {
		return nil, FieldError("id", "项目编号不能为空")
	}
	if strings.TrimSpace(title) == "" {
		return nil, FieldError("title", "节目名称不能为空")
	}
	if err := rules.Validate(); err != nil {
		return nil, err
	}
	now = now.UTC()
	return &CaptionProject{ID: id, Title: strings.TrimSpace(title), Rules: rules, Status: StatusDraft, Version: 1, CreatedAt: now, UpdatedAt: now}, nil
}

func (p *CaptionProject) Transition(to ProjectStatus, now time.Time) error {
	if err := EnsureMutable(p.Status); err != nil {
		return err
	}
	if err := ValidateTransition(p.Status, to); err != nil {
		return err
	}
	p.Status, p.UpdatedAt = to, now.UTC()
	return nil
}

func (p *CaptionProject) UpdateDraftRules(title string, rules ProjectRules, revisionCount int) ([]string, error) {
	if p.Status != StatusDraft || revisionCount != 0 {
		return nil, NewError(CodeConflict, "仅尚无字幕修订的草稿项目可修改交付规则")
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, FieldError("title", "节目名称不能为空")
	}
	rules.Language = strings.TrimSpace(rules.Language)
	rules.DeliveryStandard = strings.TrimSpace(rules.DeliveryStandard)
	if err := rules.Validate(); err != nil {
		return nil, err
	}
	changed := make([]string, 0, 6)
	if p.Title != title {
		changed = append(changed, fmt.Sprintf("title:%s->%s", p.Title, title))
	}
	if p.Rules.DurationMillis != rules.DurationMillis {
		changed = append(changed, fmt.Sprintf("durationMillis:%d->%d", p.Rules.DurationMillis, rules.DurationMillis))
	}
	if p.Rules.Language != rules.Language {
		changed = append(changed, fmt.Sprintf("language:%s->%s", p.Rules.Language, rules.Language))
	}
	if p.Rules.FrameRate != rules.FrameRate {
		changed = append(changed, fmt.Sprintf("frameRate:%g->%g", p.Rules.FrameRate, rules.FrameRate))
	}
	if p.Rules.MaxCharsPerSecond != rules.MaxCharsPerSecond {
		changed = append(changed, fmt.Sprintf("maxCharsPerSecond:%g->%g", p.Rules.MaxCharsPerSecond, rules.MaxCharsPerSecond))
	}
	if p.Rules.DeliveryStandard != rules.DeliveryStandard {
		changed = append(changed, fmt.Sprintf("deliveryStandard:%s->%s", p.Rules.DeliveryStandard, rules.DeliveryStandard))
	}
	p.Title, p.Rules = title, rules
	return changed, nil
}
