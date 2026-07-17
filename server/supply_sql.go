package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// SupplySQLStore persists the purchase workflow in the MySQL 8 schema from deploy/mysql/init.sql.
type SupplySQLStore struct{ db *sql.DB }

func NewSupplySQLStore(db *sql.DB) *SupplySQLStore { return &SupplySQLStore{db: db} }

func (s *SupplySQLStore) ListPurchaseRequests(ctx context.Context, page, pageSize int, keyword, status string) ([]PurchaseRequest, int, error) {
	page, pageSize = normalizePage(page, pageSize)
	where := []string{"1=1"}
	args := []any{}
	if strings.TrimSpace(keyword) != "" {
		where = append(where, "(id LIKE ? OR requester LIKE ? OR reason LIKE ?)")
		term := "%" + strings.TrimSpace(keyword) + "%"
		args = append(args, term, term, term)
	}
	if strings.TrimSpace(status) != "" {
		where = append(where, "status=?")
		args = append(args, status)
	}
	condition := strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM purchase_requests WHERE "+condition, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, "SELECT id,requester,department,reason,currency,CAST(estimated_amount AS CHAR),status,created_at,updated_at FROM purchase_requests WHERE "+condition+" ORDER BY created_at DESC LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []PurchaseRequest{}
	for rows.Next() {
		var item PurchaseRequest
		if err := rows.Scan(&item.ID, &item.Requester, &item.Department, &item.Reason, &item.Currency, &item.EstimatedAmount, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}

func (s *SupplySQLStore) GetPurchaseDetail(ctx context.Context, id string) (PurchaseRequestDetail, error) {
	var detail PurchaseRequestDetail
	err := s.db.QueryRowContext(ctx, `SELECT id,requester,department,reason,currency,CAST(estimated_amount AS CHAR),status,created_at,updated_at FROM purchase_requests WHERE id=?`, id).Scan(&detail.ID, &detail.Requester, &detail.Department, &detail.Reason, &detail.Currency, &detail.EstimatedAmount, &detail.Status, &detail.CreatedAt, &detail.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return PurchaseRequestDetail{}, ErrNotFound
	}
	if err != nil {
		return PurchaseRequestDetail{}, err
	}
	var scanErr error
	detail.Items, scanErr = s.queryItems(ctx, id)
	if scanErr != nil {
		return PurchaseRequestDetail{}, scanErr
	}
	detail.Quotes, scanErr = s.queryQuotes(ctx, id)
	if scanErr != nil {
		return PurchaseRequestDetail{}, scanErr
	}
	detail.Receipts, scanErr = s.queryReceipts(ctx, id)
	if scanErr != nil {
		return PurchaseRequestDetail{}, scanErr
	}
	detail.QualityChecks, scanErr = s.queryChecks(ctx, id)
	if scanErr != nil {
		return PurchaseRequestDetail{}, scanErr
	}
	detail.StockEntries, scanErr = s.queryStock(ctx, id)
	if scanErr != nil {
		return PurchaseRequestDetail{}, scanErr
	}
	detail.Events, scanErr = s.queryEvents(ctx, id)
	if scanErr != nil {
		return PurchaseRequestDetail{}, scanErr
	}
	var order PurchaseOrder
	err = s.db.QueryRowContext(ctx, `SELECT id,request_id,supplier,CAST(amount AS CHAR),status,created_at FROM purchase_orders WHERE request_id=?`, id).Scan(&order.ID, &order.RequestID, &order.Supplier, &order.Amount, &order.Status, &order.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	} else if err == nil {
		detail.Order = &order
	}
	if err != nil {
		return PurchaseRequestDetail{}, err
	}
	return detail, nil
}

func (s *SupplySQLStore) queryItems(ctx context.Context, id string) ([]PurchaseItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,request_id,sku,name,quantity,unit,CAST(unit_price AS CHAR) FROM purchase_items WHERE request_id=? ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PurchaseItem{}
	for rows.Next() {
		var item PurchaseItem
		if err := rows.Scan(&item.ID, &item.RequestID, &item.SKU, &item.Name, &item.Quantity, &item.Unit, &item.UnitPrice); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *SupplySQLStore) queryQuotes(ctx context.Context, id string) ([]SupplierQuote, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,request_id,supplier,CAST(amount AS CHAR),delivery_days,status,created_at FROM supplier_quotes WHERE request_id=? ORDER BY amount ASC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SupplierQuote{}
	for rows.Next() {
		var item SupplierQuote
		if err := rows.Scan(&item.ID, &item.RequestID, &item.Supplier, &item.Amount, &item.DeliveryDays, &item.Status, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *SupplySQLStore) queryReceipts(ctx context.Context, id string) ([]Receipt, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,request_id,COALESCE(order_id,''),quantity,COALESCE(warehouse,''),received_at,status FROM goods_receipts WHERE request_id=? ORDER BY received_at`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Receipt{}
	for rows.Next() {
		var item Receipt
		if err := rows.Scan(&item.ID, &item.RequestID, &item.OrderID, &item.Quantity, &item.Warehouse, &item.ReceivedAt, &item.Status); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *SupplySQLStore) queryChecks(ctx context.Context, id string) ([]QualityCheck, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,request_id,COALESCE(receipt_id,''),passed,COALESCE(note,''),checked_at FROM quality_checks WHERE request_id=? ORDER BY checked_at`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []QualityCheck{}
	for rows.Next() {
		var item QualityCheck
		if err := rows.Scan(&item.ID, &item.RequestID, &item.ReceiptID, &item.Passed, &item.Note, &item.CheckedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *SupplySQLStore) queryStock(ctx context.Context, id string) ([]StockEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,request_id,sku,quantity,warehouse,created_at FROM stock_entries WHERE request_id=? ORDER BY created_at`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StockEntry{}
	for rows.Next() {
		var item StockEntry
		if err := rows.Scan(&item.ID, &item.RequestID, &item.SKU, &item.Quantity, &item.Warehouse, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *SupplySQLStore) queryEvents(ctx context.Context, id string) ([]PurchaseEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,request_id,COALESCE(from_status,''),to_status,actor,COALESCE(note,''),created_at FROM purchase_events WHERE request_id=? ORDER BY created_at,id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PurchaseEvent{}
	for rows.Next() {
		var item PurchaseEvent
		if err := rows.Scan(&item.ID, &item.RequestID, &item.FromStatus, &item.ToStatus, &item.Actor, &item.Note, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *SupplySQLStore) CreatePurchaseRequest(ctx context.Context, request PurchaseRequest, inputs []PurchaseItemInput) (PurchaseRequest, error) {
	if request.ID == "" {
		request.ID = fmt.Sprintf("PR-%d", time.Now().UnixNano())
	}
	if request.Status == "" {
		request.Status = SupplyDraft
	}
	if request.Currency == "" {
		request.Currency = "CNY"
	}
	if request.CreatedAt == "" {
		request.CreatedAt = nowUTC()
	}
	request.UpdatedAt = request.CreatedAt
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PurchaseRequest{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO purchase_requests (id,requester,department,reason,currency,estimated_amount,status,created_at,updated_at) VALUES (?,?,?,?,?,?,?, ?,?)`, request.ID, request.Requester, request.Department, request.Reason, request.Currency, decimalOrZero(request.EstimatedAmount), request.Status, request.CreatedAt, request.UpdatedAt); err != nil {
		return PurchaseRequest{}, err
	}
	for i, item := range inputs {
		itemID := fmt.Sprintf("%s-I%d", request.ID, i+1)
		if _, err = tx.ExecContext(ctx, `INSERT INTO purchase_items (id,request_id,sku,name,quantity,unit,unit_price) VALUES (?,?,?,?,?,?,?)`, itemID, request.ID, item.SKU, item.Name, item.Quantity, item.Unit, decimalOrZero(item.UnitPrice)); err != nil {
			return PurchaseRequest{}, err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO purchase_events (id,request_id,from_status,to_status,actor,note,created_at) VALUES (?,?,?,?,?,?,?)`, fmt.Sprintf("PREV-%d", time.Now().UnixNano()), request.ID, "", request.Status, "申请人", "", request.CreatedAt); err != nil {
		return PurchaseRequest{}, err
	}
	if err = tx.Commit(); err != nil {
		return PurchaseRequest{}, err
	}
	return request, nil
}

func decimalOrZero(value string) string {
	if strings.TrimSpace(value) == "" {
		return "0.00"
	}
	return value
}

func (s *SupplySQLStore) transitionTx(ctx context.Context, tx *sql.Tx, id, to, actor, note string) error {
	var from string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM purchase_requests WHERE id=? FOR UPDATE`, id).Scan(&from); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if from == to {
		return nil
	}
	allowed := map[string]map[string]bool{SupplyDraft: {SupplyQuoting: true}, SupplyQuoting: {SupplyApproving: true}, SupplyApproving: {SupplyOrdered: true}, SupplyOrdered: {SupplyPartial: true}, SupplyPartial: {SupplyQCed: true}, SupplyQCed: {SupplyStocked: true}}
	if !allowed[from][to] {
		return ErrSupplyInvalidTransition
	}
	now := nowUTC()
	if _, err := tx.ExecContext(ctx, `UPDATE purchase_requests SET status=?,updated_at=? WHERE id=?`, to, now, id); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO purchase_events (id,request_id,from_status,to_status,actor,note,created_at) VALUES (?,?,?,?,?,?,?)`, fmt.Sprintf("PREV-%d", time.Now().UnixNano()), id, from, to, actor, note, now)
	return err
}
func (s *SupplySQLStore) AddSupplierQuote(ctx context.Context, id string, in AddSupplierQuoteInput) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var status string
	if e = tx.QueryRowContext(ctx, `SELECT status FROM purchase_requests WHERE id=? FOR UPDATE`, id).Scan(&status); errors.Is(e, sql.ErrNoRows) {
		return ErrNotFound
	} else if e != nil {
		return e
	}
	if status != SupplyDraft && status != SupplyQuoting {
		return ErrSupplyInvalidTransition
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO supplier_quotes (id,request_id,supplier,amount,delivery_days,status,created_at) VALUES (?,?,?,?,?,?,?)`, fmt.Sprintf("QUOTE-%d", time.Now().UnixNano()), id, in.Supplier, decimalOrZero(in.Amount), in.DeliveryDays, "待比较", nowUTC()); e != nil {
		return e
	}
	if status == SupplyDraft {
		if e = s.transitionTx(ctx, tx, id, SupplyQuoting, "采购员", "已收到供应商报价"); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func (s *SupplySQLStore) ApprovePurchaseRequest(ctx context.Context, id string) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = s.transitionTx(ctx, tx, id, SupplyApproving, "采购负责人", "比价完成，提交审批"); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *SupplySQLStore) OrderPurchaseRequest(ctx context.Context, id string) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = s.transitionTx(ctx, tx, id, SupplyOrdered, "采购负责人", "审批通过，生成采购订单"); e != nil {
		return e
	}
	var supplier, amount string
	if e = tx.QueryRowContext(ctx, `SELECT supplier,CAST(amount AS CHAR) FROM supplier_quotes WHERE request_id=? ORDER BY amount ASC LIMIT 1`, id).Scan(&supplier, &amount); errors.Is(e, sql.ErrNoRows) {
		supplier, amount = "待定", "0.00"
	} else if e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO purchase_orders (id,request_id,supplier,amount,status,created_at) VALUES (?,?,?,?,?,?) ON DUPLICATE KEY UPDATE status=VALUES(status)`, fmt.Sprintf("PO-%d", time.Now().UnixNano()), id, supplier, decimalOrZero(amount), "执行中", nowUTC()); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *SupplySQLStore) ReceivePurchaseRequest(ctx context.Context, id string, in ReceivePurchaseInput) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = s.transitionTx(ctx, tx, id, SupplyPartial, "仓库收货员", "登记到货批次"); e != nil {
		return e
	}
	var orderID string
	if e = tx.QueryRowContext(ctx, `SELECT id FROM purchase_orders WHERE request_id=?`, id).Scan(&orderID); errors.Is(e, sql.ErrNoRows) {
		orderID = ""
	} else if e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO goods_receipts (id,request_id,order_id,quantity,warehouse,received_at,status) VALUES (?,?,?,?,?,?,?)`, fmt.Sprintf("GRN-%d", time.Now().UnixNano()), id, orderID, in.Quantity, in.Warehouse, nowUTC(), "待质检"); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *SupplySQLStore) ReconcilePurchaseRequest(ctx context.Context, id string, in ReconcileInput) error {
	if !in.Passed {
		return fmt.Errorf("质检未通过")
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var status string
	if e = tx.QueryRowContext(ctx, `SELECT status FROM purchase_requests WHERE id=? FOR UPDATE`, id).Scan(&status); errors.Is(e, sql.ErrNoRows) {
		return ErrNotFound
	} else if e != nil {
		return e
	}
	if status == SupplyPartial {
		var receiptID string
		if e = tx.QueryRowContext(ctx, `SELECT id FROM goods_receipts WHERE request_id=? ORDER BY received_at DESC LIMIT 1`, id).Scan(&receiptID); errors.Is(e, sql.ErrNoRows) {
			receiptID = ""
		} else if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO quality_checks (id,request_id,receipt_id,passed,note,checked_at) VALUES (?,?,?,?,?,?)`, fmt.Sprintf("QC-%d", time.Now().UnixNano()), id, receiptID, true, in.Note, nowUTC()); e != nil {
			return e
		}
		if e = s.transitionTx(ctx, tx, id, SupplyQCed, "质检员", "质检合格"); e != nil {
			return e
		}
		return tx.Commit()
	}
	if status == SupplyQCed {
		rows, e := tx.QueryContext(ctx, `SELECT sku,quantity FROM purchase_items WHERE request_id=?`, id)
		if e != nil {
			return e
		}
		items := []struct {
			sku      string
			quantity int
		}{}
		for rows.Next() {
			var sku string
			var quantity int
			if e = rows.Scan(&sku, &quantity); e != nil {
				rows.Close()
				return e
			}
			items = append(items, struct {
				sku      string
				quantity int
			}{sku: sku, quantity: quantity})
		}
		if e = rows.Err(); e != nil {
			rows.Close()
			return e
		}
		rows.Close()
		for _, item := range items {
			if _, e = tx.ExecContext(ctx, `INSERT INTO stock_entries (id,request_id,sku,quantity,warehouse,created_at) VALUES (?,?,?,?,?,?)`, fmt.Sprintf("STK-%d", time.Now().UnixNano()), id, item.sku, item.quantity, "华东一号仓", nowUTC()); e != nil {
				return e
			}
		}
		if e = s.transitionTx(ctx, tx, id, SupplyStocked, "仓库管理员", "库存已核销"); e != nil {
			return e
		}
		return tx.Commit()
	}
	return ErrSupplyInvalidTransition
}
func (s *SupplySQLStore) ListPurchaseEvents(ctx context.Context, id string) ([]PurchaseEvent, error) {
	if _, e := s.GetPurchaseDetail(ctx, id); e != nil {
		return nil, e
	}
	return s.queryEvents(ctx, id)
}
