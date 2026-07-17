package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	SupplyDraft     = "草稿"
	SupplyQuoting   = "询价中"
	SupplyApproving = "待审批"
	SupplyOrdered   = "已下单"
	SupplyPartial   = "部分到货"
	SupplyQCed      = "已质检"
	SupplyStocked   = "已入库"
)

var ErrSupplyInvalidTransition = fmt.Errorf("采购单状态不可推进")

type PurchaseRequest struct {
	ID              string `json:"id"`
	Requester       string `json:"requester"`
	Department      string `json:"department"`
	Reason          string `json:"reason"`
	Currency        string `json:"currency"`
	EstimatedAmount string `json:"estimatedAmount"`
	Status          string `json:"status"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}
type PurchaseItem struct {
	ID        string `json:"id"`
	RequestID string `json:"requestId"`
	SKU       string `json:"sku"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	Unit      string `json:"unit"`
	UnitPrice string `json:"unitPrice"`
}
type SupplierQuote struct {
	ID           string `json:"id"`
	RequestID    string `json:"requestId"`
	Supplier     string `json:"supplier"`
	Amount       string `json:"amount"`
	DeliveryDays int    `json:"deliveryDays"`
	Status       string `json:"status"`
	CreatedAt    string `json:"createdAt"`
}
type PurchaseOrder struct {
	ID        string `json:"id"`
	RequestID string `json:"requestId"`
	Supplier  string `json:"supplier"`
	Amount    string `json:"amount"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}
type Receipt struct {
	ID         string `json:"id"`
	RequestID  string `json:"requestId"`
	OrderID    string `json:"orderId"`
	Quantity   int    `json:"quantity"`
	ReceivedAt string `json:"receivedAt"`
	Status     string `json:"status"`
	Warehouse  string `json:"warehouse,omitempty"`
}
type QualityCheck struct {
	ID        string `json:"id"`
	RequestID string `json:"requestId"`
	ReceiptID string `json:"receiptId"`
	Passed    bool   `json:"passed"`
	Note      string `json:"note"`
	CheckedAt string `json:"checkedAt"`
}
type StockEntry struct {
	ID        string `json:"id"`
	RequestID string `json:"requestId"`
	SKU       string `json:"sku"`
	Quantity  int    `json:"quantity"`
	Warehouse string `json:"warehouse"`
	CreatedAt string `json:"createdAt"`
}
type PurchaseEvent struct {
	ID         string `json:"id"`
	RequestID  string `json:"requestId"`
	FromStatus string `json:"fromStatus"`
	ToStatus   string `json:"toStatus"`
	Actor      string `json:"actor"`
	Note       string `json:"note,omitempty"`
	CreatedAt  string `json:"createdAt"`
}
type PurchaseRequestDetail struct {
	PurchaseRequest
	Items         []PurchaseItem  `json:"items"`
	Quotes        []SupplierQuote `json:"quotes"`
	Order         *PurchaseOrder  `json:"order,omitempty"`
	Receipts      []Receipt       `json:"receipts"`
	QualityChecks []QualityCheck  `json:"qualityChecks"`
	StockEntries  []StockEntry    `json:"stockEntries"`
	Events        []PurchaseEvent `json:"events"`
}
type PurchaseItemInput struct {
	SKU       string `json:"sku"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	Unit      string `json:"unit"`
	UnitPrice string `json:"unitPrice"`
}
type CreatePurchaseRequestInput struct {
	Requester  string              `json:"requester"`
	Department string              `json:"department"`
	Reason     string              `json:"reason"`
	Currency   string              `json:"currency"`
	Items      []PurchaseItemInput `json:"items"`
}
type AddSupplierQuoteInput struct {
	Supplier     string `json:"supplier"`
	Amount       string `json:"amount"`
	DeliveryDays int    `json:"deliveryDays"`
}
type ReceivePurchaseInput struct {
	Quantity  int    `json:"quantity"`
	Warehouse string `json:"warehouse"`
}
type ReconcileInput struct {
	Passed bool   `json:"passed"`
	Note   string `json:"note"`
}

type SupplyStore interface {
	ListPurchaseRequests(context.Context, int, int, string, string) ([]PurchaseRequest, int, error)
	GetPurchaseDetail(context.Context, string) (PurchaseRequestDetail, error)
	CreatePurchaseRequest(context.Context, PurchaseRequest, []PurchaseItemInput) (PurchaseRequest, error)
	AddSupplierQuote(context.Context, string, AddSupplierQuoteInput) error
	ApprovePurchaseRequest(context.Context, string) error
	OrderPurchaseRequest(context.Context, string) error
	ReceivePurchaseRequest(context.Context, string, ReceivePurchaseInput) error
	ReconcilePurchaseRequest(context.Context, string, ReconcileInput) error
	ListPurchaseEvents(context.Context, string) ([]PurchaseEvent, error)
}

// selectSupplyStore keeps demo startup deterministic while routing production SQLStore through MySQL.
func selectSupplyStore(base CareStore) SupplyStore {
	if store, ok := base.(*SQLStore); ok && store.db != nil {
		return NewSupplySQLStore(store.db)
	}
	return NewSupplyMemoryStore()
}

type SupplyMemoryStore struct {
	mu       sync.RWMutex
	seq      atomic.Uint64
	requests map[string]PurchaseRequest
	items    map[string][]PurchaseItem
	quotes   map[string][]SupplierQuote
	orders   map[string]PurchaseOrder
	receipts map[string][]Receipt
	checks   map[string][]QualityCheck
	stock    map[string][]StockEntry
	events   map[string][]PurchaseEvent
}

func NewSupplyMemoryStore() *SupplyMemoryStore {
	s := &SupplyMemoryStore{requests: map[string]PurchaseRequest{}, items: map[string][]PurchaseItem{}, quotes: map[string][]SupplierQuote{}, orders: map[string]PurchaseOrder{}, receipts: map[string][]Receipt{}, checks: map[string][]QualityCheck{}, stock: map[string][]StockEntry{}, events: map[string][]PurchaseEvent{}}
	s.seq.Store(100)
	for i, status := range []string{SupplyDraft, SupplyQuoting, SupplyApproving, SupplyOrdered, SupplyPartial, SupplyQCed, SupplyStocked} {
		id := fmt.Sprintf("PR-202607-%03d", i+1)
		now := nowUTC()
		s.requests[id] = PurchaseRequest{ID: id, Requester: []string{"采购部", "门店运营", "仓储中心"}[i%3], Department: "智能家电", Reason: "安全库存补货", Currency: "CNY", EstimatedAmount: fmt.Sprintf("%d.00", (i+3)*1200), Status: status, CreatedAt: now, UpdatedAt: now}
		s.items[id] = []PurchaseItem{{ID: id + "-I1", RequestID: id, SKU: fmt.Sprintf("SKU-%03d", i+1), Name: []string{"无线吸尘器", "空气净化器", "扫地机器人"}[i%3], Quantity: 10 + i, Unit: "件", UnitPrice: fmt.Sprintf("%d.90", 99+i*20)}}
		if status != SupplyDraft {
			s.events[id] = append(s.events[id], PurchaseEvent{ID: id + "-E1", RequestID: id, FromStatus: SupplyDraft, ToStatus: status, Actor: "系统", CreatedAt: now})
		}
	}
	return s
}
func (s *SupplyMemoryStore) next(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, s.seq.Add(1))
}
func (s *SupplyMemoryStore) get(id string) (PurchaseRequest, error) {
	r, ok := s.requests[id]
	if !ok {
		return PurchaseRequest{}, ErrNotFound
	}
	return r, nil
}
func (s *SupplyMemoryStore) transition(id, to, actor, note string) error {
	r, err := s.get(id)
	if err != nil {
		return err
	}
	if r.Status == to {
		return nil
	}
	allowed := map[string]map[string]bool{SupplyDraft: {SupplyQuoting: true}, SupplyQuoting: {SupplyApproving: true}, SupplyApproving: {SupplyOrdered: true}, SupplyOrdered: {SupplyPartial: true}, SupplyPartial: {SupplyQCed: true}, SupplyQCed: {SupplyStocked: true}}
	if !allowed[r.Status][to] {
		return ErrSupplyInvalidTransition
	}
	from := r.Status
	now := nowUTC()
	r.Status = to
	r.UpdatedAt = now
	s.requests[id] = r
	s.events[id] = append(s.events[id], PurchaseEvent{ID: s.next("PREV"), RequestID: id, FromStatus: from, ToStatus: to, Actor: actor, Note: note, CreatedAt: now})
	return nil
}
func (s *SupplyMemoryStore) ListPurchaseRequests(_ context.Context, page, pageSize int, keyword, status string) ([]PurchaseRequest, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []PurchaseRequest{}
	keyword = strings.TrimSpace(keyword)
	for _, r := range s.requests {
		if status != "" && r.Status != status {
			continue
		}
		if keyword != "" && !strings.Contains(r.ID, keyword) && !strings.Contains(r.Requester, keyword) && !strings.Contains(r.Reason, keyword) {
			continue
		}
		out = append(out, r)
	}
	page, pageSize = normalizePage(page, pageSize)
	total := len(out)
	start := (page - 1) * pageSize
	if start >= total {
		return []PurchaseRequest{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return out[start:end], total, nil
}
func (s *SupplyMemoryStore) GetPurchaseDetail(_ context.Context, id string) (PurchaseRequestDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.requests[id]
	if !ok {
		return PurchaseRequestDetail{}, ErrNotFound
	}
	d := PurchaseRequestDetail{PurchaseRequest: r, Items: append([]PurchaseItem(nil), s.items[id]...), Quotes: append([]SupplierQuote(nil), s.quotes[id]...), Receipts: append([]Receipt(nil), s.receipts[id]...), QualityChecks: append([]QualityCheck(nil), s.checks[id]...), StockEntries: append([]StockEntry(nil), s.stock[id]...), Events: append([]PurchaseEvent(nil), s.events[id]...)}
	if o, ok := s.orders[id]; ok {
		o2 := o
		d.Order = &o2
	}
	return d, nil
}
func (s *SupplyMemoryStore) CreatePurchaseRequest(_ context.Context, r PurchaseRequest, inputs []PurchaseItemInput) (PurchaseRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.ID == "" {
		r.ID = s.next("PR")
	}
	if r.Status == "" {
		r.Status = SupplyDraft
	}
	if r.Currency == "" {
		r.Currency = "CNY"
	}
	if r.CreatedAt == "" {
		r.CreatedAt = nowUTC()
	}
	r.UpdatedAt = r.CreatedAt
	s.requests[r.ID] = r
	for i, v := range inputs {
		s.items[r.ID] = append(s.items[r.ID], PurchaseItem{ID: fmt.Sprintf("%s-I%d", r.ID, i+1), RequestID: r.ID, SKU: v.SKU, Name: v.Name, Quantity: v.Quantity, Unit: v.Unit, UnitPrice: v.UnitPrice})
	}
	s.events[r.ID] = append(s.events[r.ID], PurchaseEvent{ID: s.next("PREV"), RequestID: r.ID, ToStatus: r.Status, Actor: "申请人", CreatedAt: r.CreatedAt})
	return r, nil
}
func (s *SupplyMemoryStore) AddSupplierQuote(_ context.Context, id string, in AddSupplierQuoteInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.get(id)
	if err != nil {
		return err
	}
	if r.Status != SupplyDraft && r.Status != SupplyQuoting {
		return ErrSupplyInvalidTransition
	}
	s.quotes[id] = append(s.quotes[id], SupplierQuote{ID: s.next("QUOTE"), RequestID: id, Supplier: in.Supplier, Amount: in.Amount, DeliveryDays: in.DeliveryDays, Status: "待比较", CreatedAt: nowUTC()})
	if r.Status == SupplyDraft {
		if err := s.transition(id, SupplyQuoting, "采购员", "已收到供应商报价"); err != nil {
			return err
		}
	}
	return nil
}
func (s *SupplyMemoryStore) ApprovePurchaseRequest(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.transition(id, SupplyApproving, "采购负责人", "比价完成，提交审批")
}
func (s *SupplyMemoryStore) OrderPurchaseRequest(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.transition(id, SupplyOrdered, "采购负责人", "审批通过，生成采购订单"); err != nil {
		return err
	}
	qs := s.quotes[id]
	supplier, amount := "待定", "0.00"
	if len(qs) > 0 {
		supplier, amount = qs[0].Supplier, qs[0].Amount
	}
	s.orders[id] = PurchaseOrder{ID: s.next("PO"), RequestID: id, Supplier: supplier, Amount: amount, Status: "执行中", CreatedAt: nowUTC()}
	return nil
}
func (s *SupplyMemoryStore) ReceivePurchaseRequest(_ context.Context, id string, in ReceivePurchaseInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.transition(id, SupplyPartial, "仓库收货员", "登记到货批次"); err != nil {
		return err
	}
	order := s.orders[id]
	s.receipts[id] = append(s.receipts[id], Receipt{ID: s.next("GRN"), RequestID: id, OrderID: order.ID, Quantity: in.Quantity, Warehouse: in.Warehouse, ReceivedAt: nowUTC(), Status: "待质检"})
	return nil
}
func (s *SupplyMemoryStore) ReconcilePurchaseRequest(_ context.Context, id string, in ReconcileInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !in.Passed {
		return fmt.Errorf("质检未通过")
	}
	r, err := s.get(id)
	if err != nil {
		return err
	}
	if r.Status == SupplyPartial {
		s.checks[id] = append(s.checks[id], QualityCheck{ID: s.next("QC"), RequestID: id, ReceiptID: lastReceiptID(s.receipts[id]), Passed: true, Note: in.Note, CheckedAt: nowUTC()})
		return s.transition(id, SupplyQCed, "质检员", "质检合格")
	}
	if r.Status == SupplyQCed {
		for _, item := range s.items[id] {
			s.stock[id] = append(s.stock[id], StockEntry{ID: s.next("STK"), RequestID: id, SKU: item.SKU, Quantity: item.Quantity, Warehouse: "华东一号仓", CreatedAt: nowUTC()})
		}
		return s.transition(id, SupplyStocked, "仓库管理员", "库存已核销")
	}
	return ErrSupplyInvalidTransition
}
func lastReceiptID(v []Receipt) string {
	if len(v) == 0 {
		return ""
	}
	return v[len(v)-1].ID
}
func (s *SupplyMemoryStore) ListPurchaseEvents(_ context.Context, id string) ([]PurchaseEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.requests[id]; !ok {
		return nil, ErrNotFound
	}
	return append([]PurchaseEvent(nil), s.events[id]...), nil
}

type SupplyService struct {
	store SupplyStore
	idem  idempotencyStore
}

func NewSupplyService(store SupplyStore, idem idempotencyStore) *SupplyService {
	return &SupplyService{store: store, idem: idem}
}
func (s *SupplyService) once(ctx context.Context, key, prefix, id string, action func() (string, error)) (PurchaseRequest, error) {
	if strings.TrimSpace(key) == "" {
		return PurchaseRequest{}, ErrMissingIdempotencyKey
	}
	rk := prefix + ":" + id + ":" + key
	if old, ok, e := s.idem.Get(ctx, rk); e != nil {
		return PurchaseRequest{}, e
	} else if ok {
		d, e := s.store.GetPurchaseDetail(ctx, old)
		return d.PurchaseRequest, e
	}
	release, e := s.idem.Lock(ctx, rk, 10*time.Second)
	if e != nil {
		return PurchaseRequest{}, e
	}
	defer release()
	if old, ok, e := s.idem.Get(ctx, rk); e != nil {
		return PurchaseRequest{}, e
	} else if ok {
		d, e := s.store.GetPurchaseDetail(ctx, old)
		return d.PurchaseRequest, e
	}
	rid, e := action()
	if e != nil {
		return PurchaseRequest{}, e
	}
	if e = s.idem.Set(ctx, rk, rid, 24*time.Hour); e != nil {
		return PurchaseRequest{}, e
	}
	d, e := s.store.GetPurchaseDetail(ctx, rid)
	return d.PurchaseRequest, e
}
func (s *SupplyService) CreatePurchaseRequest(ctx context.Context, in CreatePurchaseRequestInput, key string) (PurchaseRequest, error) {
	if strings.TrimSpace(in.Requester) == "" {
		return PurchaseRequest{}, fmt.Errorf("%w: requester required", ErrInvalidInput)
	}
	return s.once(ctx, key, "supply:create", "", func() (string, error) {
		r, e := s.store.CreatePurchaseRequest(ctx, PurchaseRequest{Requester: in.Requester, Department: in.Department, Reason: in.Reason, Currency: in.Currency}, in.Items)
		return r.ID, e
	})
}
func (s *SupplyService) mutate(ctx context.Context, id, key, prefix string, fn func() error) (PurchaseRequest, error) {
	return s.once(ctx, key, prefix, id, func() (string, error) {
		if e := fn(); e != nil {
			return "", e
		}
		return id, nil
	})
}
func (s *SupplyService) AddSupplierQuote(ctx context.Context, id string, in AddSupplierQuoteInput, key string) (PurchaseRequest, error) {
	if strings.TrimSpace(in.Supplier) == "" || strings.TrimSpace(in.Amount) == "" {
		return PurchaseRequest{}, fmt.Errorf("%w: supplier and amount required", ErrInvalidInput)
	}
	return s.mutate(ctx, id, key, "supply:quote", func() error { return s.store.AddSupplierQuote(ctx, id, in) })
}
func (s *SupplyService) ApprovePurchaseRequest(ctx context.Context, id, key string) (PurchaseRequest, error) {
	return s.mutate(ctx, id, key, "supply:approve", func() error { return s.store.ApprovePurchaseRequest(ctx, id) })
}
func (s *SupplyService) OrderPurchaseRequest(ctx context.Context, id, key string) (PurchaseRequest, error) {
	return s.mutate(ctx, id, key, "supply:order", func() error { return s.store.OrderPurchaseRequest(ctx, id) })
}
func (s *SupplyService) ReceivePurchaseRequest(ctx context.Context, id string, in ReceivePurchaseInput, key string) (PurchaseRequest, error) {
	if in.Quantity <= 0 {
		return PurchaseRequest{}, fmt.Errorf("%w: quantity must be positive", ErrInvalidInput)
	}
	return s.mutate(ctx, id, key, "supply:receive", func() error { return s.store.ReceivePurchaseRequest(ctx, id, in) })
}
func (s *SupplyService) ReconcilePurchaseRequest(ctx context.Context, id string, in ReconcileInput, key string) (PurchaseRequest, error) {
	return s.mutate(ctx, id, key, "supply:reconcile", func() error { return s.store.ReconcilePurchaseRequest(ctx, id, in) })
}

func registerSupplyRoutes(api *gin.RouterGroup, svc *SupplyService) {
	api.GET("/purchase-requests", func(c *gin.Context) {
		page, size := pageParams(c)
		list, total, e := svc.store.ListPurchaseRequests(c.Request.Context(), page, size, c.Query("keyword"), c.Query("status"))
		if e != nil {
			fail(c, e)
			return
		}
		respond(c, http.StatusOK, pageData(list, total, page, size))
	})
	api.GET("/purchase-requests/:id", func(c *gin.Context) {
		d, e := svc.store.GetPurchaseDetail(c.Request.Context(), c.Param("id"))
		if e != nil {
			fail(c, e)
			return
		}
		respond(c, http.StatusOK, d)
	})
	api.GET("/purchase-requests/:id/events", func(c *gin.Context) {
		ev, e := svc.store.ListPurchaseEvents(c.Request.Context(), c.Param("id"))
		if e != nil {
			fail(c, e)
			return
		}
		respond(c, http.StatusOK, gin.H{"list": ev, "total": len(ev)})
	})
	api.POST("/purchase-requests", func(c *gin.Context) {
		var in CreatePurchaseRequestInput
		if e := c.ShouldBindJSON(&in); e != nil {
			fail(c, fmt.Errorf("%w: %v", ErrInvalidInput, e))
			return
		}
		r, e := svc.CreatePurchaseRequest(c.Request.Context(), in, c.GetHeader("Idempotency-Key"))
		if e != nil {
			fail(c, e)
			return
		}
		respond(c, http.StatusCreated, r)
	})
	api.POST("/purchase-requests/:id/quotes", func(c *gin.Context) {
		var in AddSupplierQuoteInput
		if e := c.ShouldBindJSON(&in); e != nil {
			fail(c, fmt.Errorf("%w: %v", ErrInvalidInput, e))
			return
		}
		r, e := svc.AddSupplierQuote(c.Request.Context(), c.Param("id"), in, c.GetHeader("Idempotency-Key"))
		if e != nil {
			fail(c, e)
			return
		}
		respond(c, http.StatusOK, r)
	})
	api.POST("/purchase-requests/:id/approve", func(c *gin.Context) {
		r, e := svc.ApprovePurchaseRequest(c.Request.Context(), c.Param("id"), c.GetHeader("Idempotency-Key"))
		if e != nil {
			fail(c, e)
			return
		}
		respond(c, http.StatusOK, r)
	})
	api.POST("/purchase-requests/:id/order", func(c *gin.Context) {
		r, e := svc.OrderPurchaseRequest(c.Request.Context(), c.Param("id"), c.GetHeader("Idempotency-Key"))
		if e != nil {
			fail(c, e)
			return
		}
		respond(c, http.StatusOK, r)
	})
	api.POST("/purchase-requests/:id/receipt", func(c *gin.Context) {
		var in ReceivePurchaseInput
		if e := c.ShouldBindJSON(&in); e != nil {
			fail(c, fmt.Errorf("%w: %v", ErrInvalidInput, e))
			return
		}
		r, e := svc.ReceivePurchaseRequest(c.Request.Context(), c.Param("id"), in, c.GetHeader("Idempotency-Key"))
		if e != nil {
			fail(c, e)
			return
		}
		respond(c, http.StatusOK, r)
	})
	api.POST("/purchase-requests/:id/reconcile", func(c *gin.Context) {
		var in ReconcileInput
		if e := c.ShouldBindJSON(&in); e != nil {
			fail(c, fmt.Errorf("%w: %v", ErrInvalidInput, e))
			return
		}
		r, e := svc.ReconcilePurchaseRequest(c.Request.Context(), c.Param("id"), in, c.GetHeader("Idempotency-Key"))
		if e != nil {
			fail(c, e)
			return
		}
		respond(c, http.StatusOK, r)
	})
}
