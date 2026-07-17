package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouterAppointmentLifecycleAndEnvelope(t *testing.T) {
	r := NewRouter(NewMemoryStore(), newMemoryIdempotency())
	body := bytes.NewBufferString(`{"patient":"案卷 C001","department":"合同纠纷","doctor":"林律师"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/appointments", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "handler-create")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body=%s", res.Code, res.Body.String())
	}
	var envelope struct {
		Code    int         `json:"code"`
		TraceID string      `json:"traceId"`
		Data    Appointment `json:"data"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != 0 || envelope.TraceID == "" || envelope.Data.ID == "" {
		t.Fatalf("bad envelope: %+v", envelope)
	}
	statusReq := httptest.NewRequest(http.MethodPost, "/api/v1/appointments/"+envelope.Data.ID+"/status", bytes.NewBufferString(`{"status":"已立案"}`))
	statusReq.Header.Set("Content-Type", "application/json")
	statusReq.Header.Set("Idempotency-Key", "handler-checkin")
	statusRes := httptest.NewRecorder()
	r.ServeHTTP(statusRes, statusReq)
	if statusRes.Code != http.StatusOK {
		t.Fatalf("checkin status = %d, body=%s", statusRes.Code, statusRes.Body.String())
	}
	illegalReq := httptest.NewRequest(http.MethodPost, "/api/v1/appointments/"+envelope.Data.ID+"/status", bytes.NewBufferString(`{"status":"已结案"}`))
	illegalReq.Header.Set("Content-Type", "application/json")
	illegalReq.Header.Set("Idempotency-Key", "handler-illegal")
	illegalRes := httptest.NewRecorder()
	r.ServeHTTP(illegalRes, illegalReq)
	if illegalRes.Code != http.StatusConflict {
		t.Fatalf("illegal transition status = %d, body=%s", illegalRes.Code, illegalRes.Body.String())
	}
	eventsReq := httptest.NewRequest(http.MethodGet, "/api/v1/appointments/"+envelope.Data.ID+"/events", nil)
	eventsRes := httptest.NewRecorder()
	r.ServeHTTP(eventsRes, eventsReq)
	if eventsRes.Code != http.StatusOK || !bytes.Contains(eventsRes.Body.Bytes(), []byte("已立案")) {
		t.Fatalf("events response = %d, body=%s", eventsRes.Code, eventsRes.Body.String())
	}
}

func TestRouterRejectsWriteWithoutIdempotencyKey(t *testing.T) {
	r := NewRouter(NewMemoryStore(), newMemoryIdempotency())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/appointments", bytes.NewBufferString(`{"patient":"缺少幂等键"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", res.Code, res.Body.String())
	}
}

func TestRouterMatterWorkflowAndSensitiveFieldRejection(t *testing.T) {
	r := NewRouter(NewMemoryStore(), newMemoryIdempotency())
	bad := httptest.NewRequest(http.MethodPost, "/api/v1/matters", bytes.NewBufferString(`{"subjectAlias":"演示案卷","caseType":"合同审查","priority":"高","deadline":"2026-07-30","customerName":"真实姓名"}`))
	bad.Header.Set("Content-Type", "application/json")
	bad.Header.Set("Idempotency-Key", "matter-bad")
	badRes := httptest.NewRecorder(); r.ServeHTTP(badRes, bad)
	if badRes.Code != http.StatusBadRequest { t.Fatalf("sensitive field status=%d body=%s", badRes.Code, badRes.Body.String()) }

	create := httptest.NewRequest(http.MethodPost, "/api/v1/matters", bytes.NewBufferString(`{"subjectAlias":"演示案卷-HTTP","caseType":"合同审查","priority":"高","deadline":"2026-07-30"}`))
	create.Header.Set("Content-Type", "application/json"); create.Header.Set("Idempotency-Key", "matter-http")
	created := httptest.NewRecorder(); r.ServeHTTP(created, create)
	if created.Code != http.StatusCreated { t.Fatalf("create status=%d body=%s", created.Code, created.Body.String()) }
	var envelope struct { Data Matter `json:"data"` }; if err := json.Unmarshal(created.Body.Bytes(), &envelope); err != nil { t.Fatal(err) }
	id := envelope.Data.ID
	file := httptest.NewRequest(http.MethodPost, "/api/v1/matters/"+id+"/file", bytes.NewBufferString(`{"name":"合同.pdf","kind":"contract","checksum":"sha256:http-1"}`))
	file.Header.Set("Content-Type", "application/json"); file.Header.Set("Idempotency-Key", "file-http")
	fileRes := httptest.NewRecorder(); r.ServeHTTP(fileRes, file)
	if fileRes.Code != http.StatusCreated { t.Fatalf("file status=%d body=%s", fileRes.Code, fileRes.Body.String()) }
	assign := httptest.NewRequest(http.MethodPost, "/api/v1/matters/"+id+"/assign", bytes.NewBufferString(`{"assignee":"林律师","actor":"许汝林"}`))
	assign.Header.Set("Content-Type", "application/json"); assign.Header.Set("Idempotency-Key", "assign-http")
	assignRes := httptest.NewRecorder(); r.ServeHTTP(assignRes, assign)
	if assignRes.Code != http.StatusOK { t.Fatalf("assign status=%d body=%s", assignRes.Code, assignRes.Body.String()) }
	events := httptest.NewRequest(http.MethodGet, "/api/v1/matters/"+id+"/events", nil)
	eventsRes := httptest.NewRecorder(); r.ServeHTTP(eventsRes, events)
	if eventsRes.Code != http.StatusOK || !bytes.Contains(eventsRes.Body.Bytes(), []byte("归档文档")) { t.Fatalf("events status=%d body=%s", eventsRes.Code, eventsRes.Body.String()) }
}
