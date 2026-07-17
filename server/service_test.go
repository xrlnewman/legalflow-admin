package main

import (
	"context"
	"errors"
	"testing"
)

func TestAppointmentStatusTransitions(t *testing.T) {
	store := NewMemoryStore()
	svc := NewCareService(store, NoopIdempotency{})
	ctx := context.Background()
	appointment, err := svc.CreateAppointment(ctx, CreateAppointmentInput{Patient: "案卷 C001", Department: "合同纠纷", Doctor: "林律师", ScheduledAt: "2026-07-16T09:00:00+08:00"}, "create-1")
	if err != nil {
		t.Fatal(err)
	}
	steps := []string{"已立案", "待办理", "办理中", "已结案"}
	for _, status := range steps {
		appointment, err = svc.UpdateAppointmentStatus(ctx, appointment.ID, status, "status-"+status)
		if err != nil {
			t.Fatalf("status %s: %v", status, err)
		}
		if appointment.Status != status {
			t.Fatalf("status = %q, want %q", appointment.Status, status)
		}
	}
	if _, err := svc.UpdateAppointmentStatus(ctx, appointment.ID, "办理中", "illegal-1"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}
	events, err := store.ListAppointmentEvents(ctx, appointment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 {
		t.Fatalf("events = %d, want 4", len(events))
	}
}

func TestAppointmentWriteRequiresIdempotencyKey(t *testing.T) {
	store := NewMemoryStore()
	svc := NewCareService(store, NoopIdempotency{})
	_, err := svc.CreateAppointment(context.Background(), CreateAppointmentInput{Patient: "沈明远"}, "")
	if !errors.Is(err, ErrMissingIdempotencyKey) {
		t.Fatalf("expected missing idempotency key, got %v", err)
	}
}

func TestAppointmentWriteIsIdempotent(t *testing.T) {
	store := NewMemoryStore()
	svc := NewCareService(store, NoopIdempotency{})
	input := CreateAppointmentInput{Patient: "案卷 C002", Department: "劳动争议", Doctor: "沈律师"}
	a, err := svc.CreateAppointment(context.Background(), input, "same-key")
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.CreateAppointment(context.Background(), input, "same-key")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != b.ID {
		t.Fatalf("idempotency returned %q then %q", a.ID, b.ID)
	}
}

func TestFollowupCompletesOnce(t *testing.T) {
	store := NewMemoryStore()
	svc := NewCareService(store, NoopIdempotency{})
	followup, err := store.CreateFollowup(context.Background(), Followup{Patient: "案卷 C001", Summary: "证据清单与举证期限"})
	if err != nil {
		t.Fatal(err)
	}
	completed, err := svc.CompleteFollowup(context.Background(), followup.ID, "followup-1")
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != "已完成" {
		t.Fatalf("status = %q", completed.Status)
	}
	if _, err := svc.CompleteFollowup(context.Background(), followup.ID, "followup-2"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid completion, got %v", err)
	}
}

func TestMatterCreateIsAliasOnlyAndRequiresDeadline(t *testing.T) {
	store := NewMemoryStore()
	svc := NewCareService(store, NoopIdempotency{})
	ctx := context.Background()
	if _, err := svc.CreateMatter(ctx, CreateMatterInput{SubjectAlias: "演示案卷-001", CaseType: "合同审查", Priority: "高"}, "matter-1"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected missing deadline validation, got %v", err)
	}
	if _, err := svc.CreateMatter(ctx, CreateMatterInput{SubjectAlias: "张三", CaseType: "合同审查", Priority: "高", Deadline: "2026-07-30"}, "matter-2"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected alias-only validation, got %v", err)
	}
	for _, identifier := range []string{"13800138000", "11010519491231002X"} {
		if _, err := svc.CreateMatter(ctx, CreateMatterInput{SubjectAlias: identifier, CaseType: "合同审查", Priority: "高", Deadline: "2026-07-30"}, "matter-pii-"+identifier); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected personal identifier to be rejected: %q, got %v", identifier, err)
		}
	}
	matter, err := svc.CreateMatter(ctx, CreateMatterInput{SubjectAlias: "演示案卷-001", CaseType: "合同审查", Priority: "高", Deadline: "2026-07-30"}, "matter-3")
	if err != nil || matter.Status != MatterPending || matter.SubjectAlias != "演示案卷-001" {
		t.Fatalf("create matter = %+v, err=%v", matter, err)
	}
}

func TestMatterAssignmentOwnershipAndCloseGate(t *testing.T) {
	store := NewMemoryStore()
	svc := NewCareService(store, NoopIdempotency{})
	ctx := context.Background()
	matter, err := svc.CreateMatter(ctx, CreateMatterInput{SubjectAlias: "演示案卷-002", CaseType: "劳动争议", Priority: "中", Deadline: "2026-07-30"}, "matter-assign")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.CloseMatter(ctx, matter.ID, CloseMatterInput{Result: "已完成", Actor: "许汝林"}, "close-too-early"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected close gate, got %v", err)
	}
	assigned, task, err := svc.AssignMatter(ctx, matter.ID, AssignMatterInput{Assignee: "林律师", Actor: "许汝林"}, "assign-1")
	if err != nil || assigned.Assignee != "林律师" || assigned.Status != MatterFiled || task.Assignee != "林律师" {
		t.Fatalf("assignment = %+v task=%+v err=%v", assigned, task, err)
	}
	if _, err := svc.UpdateMatterStatus(ctx, matter.ID, MatterCollaborating, "许汝林", "status-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateMatterStatus(ctx, matter.ID, MatterPendingClose, "林律师", "status-2"); err != nil {
		t.Fatal(err)
	}
	closed, _, err := svc.CloseMatter(ctx, matter.ID, CloseMatterInput{Result: "材料已归档", Actor: "林律师"}, "close-1")
	if err != nil || closed.Status != MatterClosed || closed.ClosureResult != "材料已归档" {
		t.Fatalf("close = %+v err=%v", closed, err)
	}
}

func TestMatterDocumentChecksumIsIdempotentAndEventsOrdered(t *testing.T) {
	store := NewMemoryStore()
	svc := NewCareService(store, NoopIdempotency{})
	ctx := context.Background()
	matter, err := svc.CreateMatter(ctx, CreateMatterInput{SubjectAlias: "演示案卷-003", CaseType: "知识产权", Priority: "高", Deadline: "2026-07-30"}, "matter-doc")
	if err != nil {
		t.Fatal(err)
	}
	doc1, err := svc.AddMatterDocument(ctx, matter.ID, AddMatterFileInput{Name: "证据清单.pdf", Kind: "evidence", Checksum: "sha256:demo-001"}, "file-1")
	if err != nil {
		t.Fatal(err)
	}
	doc2, err := svc.AddMatterDocument(ctx, matter.ID, AddMatterFileInput{Name: "证据清单-重复.pdf", Kind: "evidence", Checksum: "sha256:demo-001"}, "file-2")
	if err != nil || doc1.ID != doc2.ID {
		t.Fatalf("duplicate docs = %+v / %+v err=%v", doc1, doc2, err)
	}
	events, err := store.ListMatterEvents(ctx, matter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Action != "创建案件" || events[1].Action != "归档文档" || events[0].CreatedAt > events[1].CreatedAt {
		t.Fatalf("events = %+v", events)
	}
}
