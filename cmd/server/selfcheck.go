package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"subtitle-review/internal/domain"
)

type selfcheckAggregate struct {
	Project    domain.CaptionProject      `json:"project"`
	Revisions  []domain.CaptionRevision   `json:"revisions"`
	Issues     []domain.ValidationIssue   `json:"issues"`
	Credential *domain.DeliveryCredential `json:"credential"`
	Timeline   []domain.AuditEvent        `json:"timeline"`
}

func runSelfcheck(addr string) error {
	client := &http.Client{Timeout: 4 * time.Second}
	base := "http://" + addr
	var created selfcheckAggregate
	if err := selfcheckJSON(client, http.MethodPost, base+"/api/projects", map[string]any{
		"id": "selfcheck-project", "title": "自检节目", "durationMillis": 20000,
		"language": "zh-CN", "frameRate": 25, "maxCharsPerSecond": 15,
		"deliveryStandard": "公共文化无障碍字幕规范", "actor": "自检编辑", "idempotencyKey": "selfcheck-create",
	}, &created); err != nil {
		return fmt.Errorf("自检建档: %w", err)
	}
	var rulesUpdated selfcheckAggregate
	if err := selfcheckJSON(client, http.MethodPut, base+"/api/projects/selfcheck-project/rules", map[string]any{
		"expectedVersion": created.Project.Version, "title": "自检节目（规则已校准）", "durationMillis": 20000,
		"language": "zh-CN", "frameRate": 25, "maxCharsPerSecond": 15,
		"deliveryStandard": "公共文化无障碍字幕规范", "actor": "自检编辑", "idempotencyKey": "selfcheck-rules",
	}, &rulesUpdated); err != nil {
		return fmt.Errorf("自检规则修订: %w", err)
	}
	flawedCues := []map[string]any{{"sequence": 1, "startMillis": 0, "endMillis": 500, "text": "这是一条阅读速度明显超过阈值的字幕文本", "speaker": "主持人"}}
	var preflight struct {
		IssueCount int    `json:"issueCount"`
		Digest     string `json:"contentDigest"`
		Version    int64  `json:"version"`
	}
	if err := selfcheckJSON(client, http.MethodPost, base+"/api/projects/selfcheck-project/revisions/preflight", map[string]any{
		"expectedVersion": rulesUpdated.Project.Version, "submittedBy": "自检编辑", "cues": flawedCues,
	}, &preflight); err != nil {
		return fmt.Errorf("自检修订预检: %w", err)
	}
	if preflight.IssueCount == 0 || preflight.Digest == "" || preflight.Version != rulesUpdated.Project.Version {
		return fmt.Errorf("自检预检未返回预期质量结果")
	}
	var flawed selfcheckAggregate
	if err := selfcheckJSON(client, http.MethodPost, base+"/api/projects/selfcheck-project/revisions", map[string]any{
		"expectedVersion": rulesUpdated.Project.Version, "submittedBy": "自检编辑", "idempotencyKey": "selfcheck-revision-1", "cues": flawedCues,
	}, &flawed); err != nil {
		return fmt.Errorf("自检问题修订: %w", err)
	}
	if len(flawed.Issues) == 0 || flawed.Project.Status != domain.StatusFixing {
		return fmt.Errorf("自检未产生预期质量问题")
	}
	items := make([]map[string]string, 0, len(flawed.Issues))
	for _, issue := range flawed.Issues {
		items = append(items, map[string]string{"issueID": issue.ID, "disposition": "延长显示时长并精简文本"})
	}
	var batch struct {
		Aggregate selfcheckAggregate `json:"aggregate"`
	}
	if err := selfcheckJSON(client, http.MethodPost, base+"/api/projects/selfcheck-project/issues/batch-disposition", map[string]any{"expectedVersion": flawed.Project.Version, "items": items, "actor": "自检编辑", "idempotencyKey": "selfcheck-batch-disposition"}, &batch); err != nil {
		return fmt.Errorf("自检批量问题处置: %w", err)
	}
	dispositioned := batch.Aggregate
	var clean selfcheckAggregate
	if err := selfcheckJSON(client, http.MethodPost, base+"/api/projects/selfcheck-project/revisions", map[string]any{
		"expectedVersion": dispositioned.Project.Version, "submittedBy": "自检编辑", "idempotencyKey": "selfcheck-revision-2",
		"cues": []map[string]any{{"sequence": 1, "startMillis": 0, "endMillis": 3000, "text": "欢迎收看本期节目", "speaker": "主持人"}, {"sequence": 2, "startMillis": 3500, "endMillis": 7000, "text": "公共文化服务与你相伴", "speaker": ""}},
	}, &clean); err != nil {
		return fmt.Errorf("自检替代修订: %w", err)
	}
	if clean.Project.Status != domain.StatusReview {
		return fmt.Errorf("干净修订未进入待复核状态: %s", clean.Project.Status)
	}
	var diff struct {
		Counts map[string]int `json:"counts"`
	}
	diffURL := fmt.Sprintf("%s/api/projects/selfcheck-project/revisions/%s/diff", base, clean.Revisions[len(clean.Revisions)-1].ID)
	if err := selfcheckJSON(client, http.MethodGet, diffURL, nil, &diff); err != nil {
		return fmt.Errorf("自检修订差异: %w", err)
	}
	if len(diff.Counts) == 0 {
		return fmt.Errorf("自检修订差异缺少计数")
	}
	var reviewContext struct {
		Readiness struct {
			ProjectAwaitingReview bool `json:"projectAwaitingReview"`
		} `json:"readiness"`
	}
	if err := selfcheckJSON(client, http.MethodGet, base+"/api/projects/selfcheck-project/review-context?reviewer=独立审核员", nil, &reviewContext); err != nil {
		return fmt.Errorf("自检复核上下文: %w", err)
	}
	if !reviewContext.Readiness.ProjectAwaitingReview {
		return fmt.Errorf("自检复核上下文未标记待复核")
	}
	var approved selfcheckAggregate
	if err := selfcheckJSON(client, http.MethodPost, base+"/api/projects/selfcheck-project/reviews", map[string]any{"expectedVersion": clean.Project.Version, "reviewer": "独立审核员", "languageApproved": true, "accessibilityApproved": true, "decision": "approve", "reason": "复核通过", "idempotencyKey": "selfcheck-review"}, &approved); err != nil {
		return fmt.Errorf("自检人工复核: %w", err)
	}
	var readiness struct {
		Ready        bool `json:"ready"`
		BlockerCount int  `json:"blockerCount"`
	}
	readinessURL := fmt.Sprintf("%s/api/projects/selfcheck-project/freeze-readiness?expectedVersion=%d", base, approved.Project.Version)
	if err := selfcheckJSON(client, http.MethodGet, readinessURL, nil, &readiness); err != nil {
		return fmt.Errorf("自检冻结就绪: %w", err)
	}
	if !readiness.Ready || readiness.BlockerCount != 0 {
		return fmt.Errorf("自检项目未通过冻结就绪检查")
	}
	var frozen selfcheckAggregate
	if err := selfcheckJSON(client, http.MethodPost, base+"/api/projects/selfcheck-project/freeze", map[string]any{"expectedVersion": approved.Project.Version, "issuedBy": "发布负责人", "idempotencyKey": "selfcheck-freeze"}, &frozen); err != nil {
		return fmt.Errorf("自检冻结: %w", err)
	}
	if frozen.Credential == nil || frozen.Project.Status != domain.StatusFrozen {
		return fmt.Errorf("冻结未生成凭据")
	}
	var verification struct {
		Valid   bool   `json:"valid"`
		Message string `json:"message"`
		Checks  []struct {
			Passed bool `json:"passed"`
		} `json:"checks"`
	}
	if err := selfcheckJSON(client, http.MethodGet, base+"/api/verify/"+frozen.Credential.VerificationCode, nil, &verification); err != nil {
		return fmt.Errorf("自检凭据验证: %w", err)
	}
	if !verification.Valid || len(verification.Checks) != 5 {
		return fmt.Errorf("凭据验证失败: %s", verification.Message)
	}
	return nil
}

func selfcheckJSON(client *http.Client, method, url string, input, output any) error {
	var body io.Reader
	if input != nil {
		b, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}
	if output != nil && len(data) > 0 {
		if err := json.Unmarshal(data, output); err != nil {
			return err
		}
	}
	return nil
}
