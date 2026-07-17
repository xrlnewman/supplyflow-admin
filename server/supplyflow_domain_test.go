package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSupplyRequestLifecycleKeepsEventsAndIsIdempotent(t *testing.T) {
	store := NewSupplyMemoryStore()
	svc := NewSupplyService(store, newMemoryIdempotency())
	ctx := context.Background()
	r, err := svc.CreatePurchaseRequest(ctx, CreatePurchaseRequestInput{Requester: "采购部", Reason: "补充智能家电库存", Items: []PurchaseItemInput{{SKU: "VAC-001", Name: "无线吸尘器", Quantity: 12, UnitPrice: "399.90"}}}, "pr-1")
	if err != nil { t.Fatal(err) }
	if r.Status != SupplyDraft { t.Fatalf("status = %q", r.Status) }
	if again, err := svc.CreatePurchaseRequest(ctx, CreatePurchaseRequestInput{Requester: "采购部"}, "pr-1"); err != nil || again.ID != r.ID { t.Fatalf("idempotency: %+v %v", again, err) }
	if _, err = svc.AddSupplierQuote(ctx, r.ID, AddSupplierQuoteInput{Supplier: "宁波星链", Amount: "4788.80", DeliveryDays: 5}, "q-1"); err != nil { t.Fatal(err) }
	if _, err = svc.ApprovePurchaseRequest(ctx, r.ID, "approve-1"); err != nil { t.Fatal(err) }
	if _, err = svc.OrderPurchaseRequest(ctx, r.ID, "order-1"); err != nil { t.Fatal(err) }
	if _, err = svc.ReceivePurchaseRequest(ctx, r.ID, ReceivePurchaseInput{Quantity: 12}, "receive-1"); err != nil { t.Fatal(err) }
	if _, err = svc.ReconcilePurchaseRequest(ctx, r.ID, ReconcileInput{Passed: true}, "qc-1"); err != nil { t.Fatal(err) }
	final, err := svc.ReconcilePurchaseRequest(ctx, r.ID, ReconcileInput{Passed: true}, "stock-1")
	if err != nil { t.Fatal(err) }
	if final.Status != SupplyStocked { t.Fatalf("final status = %q", final.Status) }
	events, err := store.ListPurchaseEvents(ctx, r.ID)
	if err != nil || len(events) < 6 { t.Fatalf("events = %d, err=%v", len(events), err) }
}

func TestSupplyRouterListAndWriteEnvelope(t *testing.T) {
	r := NewRouter(NewMemoryStore(), newMemoryIdempotency())
	body := bytes.NewBufferString(`{"requester":"采购部","reason":"补货","items":[{"sku":"SKU-1","name":"滤芯","quantity":2,"unitPrice":"19.90"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/purchase-requests", body)
	req.Header.Set("Content-Type", "application/json"); req.Header.Set("Idempotency-Key", "router-pr")
	res := httptest.NewRecorder(); r.ServeHTTP(res, req)
	if res.Code != http.StatusCreated { t.Fatalf("status=%d body=%s", res.Code, res.Body.String()) }
	var envelope struct { Code int `json:"code"`; Data PurchaseRequest `json:"data"` }
	if err := json.Unmarshal(res.Body.Bytes(), &envelope); err != nil { t.Fatal(err) }
	if envelope.Code != 0 || envelope.Data.ID == "" { t.Fatalf("bad envelope: %+v", envelope) }
	list := httptest.NewRecorder(); r.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/purchase-requests?keyword=补货&pageSize=10", nil))
	if list.Code != http.StatusOK || !bytes.Contains(list.Body.Bytes(), []byte("补货")) { t.Fatalf("list=%d body=%s", list.Code, list.Body.String()) }
}

func TestSupplyRejectsMutationAfterStocked(t *testing.T) {
	store := NewSupplyMemoryStore(); svc := NewSupplyService(store, newMemoryIdempotency())
	r, _ := svc.CreatePurchaseRequest(context.Background(), CreatePurchaseRequestInput{Requester: "采购部"}, "x")
	for _, step := range []func() error{
		func() error { _, e := svc.AddSupplierQuote(context.Background(), r.ID, AddSupplierQuoteInput{Supplier: "供应商", Amount: "1.00"}, "q"); return e },
		func() error { _, e := svc.ApprovePurchaseRequest(context.Background(), r.ID, "a"); return e },
		func() error { _, e := svc.OrderPurchaseRequest(context.Background(), r.ID, "o"); return e },
		func() error { _, e := svc.ReceivePurchaseRequest(context.Background(), r.ID, ReceivePurchaseInput{Quantity: 1}, "r"); return e },
		func() error { _, e := svc.ReconcilePurchaseRequest(context.Background(), r.ID, ReconcileInput{Passed: true}, "c"); return e },
		func() error { _, e := svc.ReconcilePurchaseRequest(context.Background(), r.ID, ReconcileInput{Passed: true}, "s"); return e },
	} { if err := step(); err != nil { t.Fatal(err) } }
	_, err := svc.AddSupplierQuote(context.Background(), r.ID, AddSupplierQuoteInput{Supplier: "晚到", Amount: "2.00"}, "late")
	if !errors.Is(err, ErrSupplyInvalidTransition) { t.Fatalf("expected locked error, got %v", err) }
}
