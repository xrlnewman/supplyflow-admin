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
	body := bytes.NewBufferString(`{"patient":"测试用户","department":"全科门诊","doctor":"林负责人"}`)
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
	statusReq := httptest.NewRequest(http.MethodPost, "/api/v1/appointments/"+envelope.Data.ID+"/status", bytes.NewBufferString(`{"status":"已确认"}`))
	statusReq.Header.Set("Content-Type", "application/json")
	statusReq.Header.Set("Idempotency-Key", "handler-checkin")
	statusRes := httptest.NewRecorder()
	r.ServeHTTP(statusRes, statusReq)
	if statusRes.Code != http.StatusOK {
		t.Fatalf("checkin status = %d, body=%s", statusRes.Code, statusRes.Body.String())
	}
	illegalReq := httptest.NewRequest(http.MethodPost, "/api/v1/appointments/"+envelope.Data.ID+"/status", bytes.NewBufferString(`{"status":"已完成"}`))
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
	if eventsRes.Code != http.StatusOK || !bytes.Contains(eventsRes.Body.Bytes(), []byte("已确认")) {
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
