package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"subtitle-review/internal/domain"
)

func (s *FileStore) commit(next database, event domain.AuditEvent) error {
	if err := validateDatabase(next); err != nil {
		return err
	}
	b, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, "snapshot-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err = tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = appendAudit(s.auditPath, event); err != nil {
		return err
	}
	if err = os.Rename(tmpName, s.snapshotPath); err != nil {
		return err
	}
	if err = syncDirectory(s.dir); err != nil {
		return err
	}
	s.db = next
	return nil
}

func appendAudit(path string, event domain.AuditEvent) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	b, err := json.Marshal(event)
	if err == nil {
		_, err = f.Write(append(b, '\n'))
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func syncDirectory(path string) error {
	d, err := os.Open(filepath.Clean(path))
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

func (s *FileStore) reconcileAudit() error {
	events, err := readAudit(s.auditPath)
	if err != nil {
		return err
	}
	previous := ""
	seen := make(map[int64]domain.AuditEvent)
	for _, e := range events {
		if e.PreviousDigest != previous {
			return domain.NewError(domain.CodeIntegrity, "审计日志摘要链断裂")
		}
		if auditEventDigest(e) != e.Digest {
			return domain.NewError(domain.CodeIntegrity, "审计日志摘要校验失败")
		}
		previous = e.Digest
		seen[e.Sequence] = e
	}
	// 快照可能已原子提交而日志追加尚未完成，启动时从快照时间线补齐。
	missing := make([]domain.AuditEvent, 0)
	for _, agg := range s.db.Projects {
		for _, e := range agg.Timeline {
			if _, ok := seen[e.Sequence]; !ok {
				missing = append(missing, e)
			}
		}
	}
	for len(missing) > 0 {
		index := -1
		for i := range missing {
			if missing[i].Sequence == int64(len(events)+1) {
				index = i
				break
			}
		}
		if index < 0 {
			return domain.NewError(domain.CodeIntegrity, "快照与审计日志序号不一致")
		}
		e := missing[index]
		if e.PreviousDigest != previous {
			return domain.NewError(domain.CodeIntegrity, "补偿审计事件摘要链不一致")
		}
		if err := appendAudit(s.auditPath, e); err != nil {
			return err
		}
		previous = e.Digest
		events = append(events, e)
		missing = append(missing[:index], missing[index+1:]...)
	}
	if len(events) > 0 && (int64(len(events)) != s.db.AuditSequence || previous != s.db.AuditDigest) {
		return domain.NewError(domain.CodeIntegrity, "快照与审计日志末端不一致")
	}
	return nil
}

func readAudit(path string) ([]domain.AuditEvent, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var events []domain.AuditEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e domain.AuditEvent
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("解析审计日志: %w", err)
		}
		events = append(events, e)
	}
	return events, scanner.Err()
}
