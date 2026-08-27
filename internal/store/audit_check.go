package store

import (
	"fmt"
	"reflect"

	"subtitle-review/internal/domain"
)

// CheckAuditIntegrity validates the append-only audit chain without changing storage.
func (s *FileStore) CheckAuditIntegrity() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events, err := readAudit(s.auditPath)
	if err != nil {
		return err
	}
	if int64(len(events)) != s.db.AuditSequence {
		return domain.NewError(domain.CodeIntegrity, "审计日志事件数量与快照不一致")
	}
	fromSnapshot := make(map[int64]domain.AuditEvent, len(events))
	for _, agg := range s.db.Projects {
		for _, event := range agg.Timeline {
			fromSnapshot[event.Sequence] = event
		}
	}
	previous := ""
	for i, event := range events {
		if event.Sequence != int64(i+1) {
			return domain.NewError(domain.CodeIntegrity, "审计日志序号不连续")
		}
		if event.PreviousDigest != previous || auditEventDigest(event) != event.Digest {
			return domain.NewError(domain.CodeIntegrity, "审计日志摘要链校验失败")
		}
		snapshotEvent, ok := fromSnapshot[event.Sequence]
		if !ok || auditEventDigest(snapshotEvent) != snapshotEvent.Digest || !reflect.DeepEqual(snapshotEvent, event) {
			return domain.NewError(domain.CodeIntegrity, fmt.Sprintf("审计事件 %d 与快照不一致", event.Sequence))
		}
		previous = event.Digest
	}
	if previous != s.db.AuditDigest {
		return domain.NewError(domain.CodeIntegrity, "审计日志末端摘要与快照不一致")
	}
	return nil
}
