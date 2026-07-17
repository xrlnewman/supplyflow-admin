-- SupplyFlow synthetic operational data only. Never load real medical records here.
CREATE TABLE IF NOT EXISTS departments (
  id VARCHAR(64) PRIMARY KEY,
  name VARCHAR(64) NOT NULL UNIQUE
);
CREATE TABLE IF NOT EXISTS doctors (
  id VARCHAR(64) PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  department VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  today_count INT NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS patients (
  id VARCHAR(64) PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  phone VARCHAR(32) NOT NULL,
  last_visit VARCHAR(32) NOT NULL,
  created_at VARCHAR(64) NOT NULL
);
CREATE TABLE IF NOT EXISTS appointments (
  id VARCHAR(64) PRIMARY KEY,
  patient_id VARCHAR(64) NOT NULL,
  patient_name VARCHAR(64) NOT NULL,
  department VARCHAR(64) NOT NULL,
  doctor VARCHAR(64) NOT NULL,
  scheduled_at VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  created_at VARCHAR(64) NOT NULL,
  updated_at VARCHAR(64) NOT NULL,
  INDEX idx_appointments_status_time (status, scheduled_at)
);
CREATE TABLE IF NOT EXISTS appointment_events (
  id VARCHAR(64) PRIMARY KEY,
  appointment_id VARCHAR(64) NOT NULL,
  from_status VARCHAR(32) NOT NULL,
  to_status VARCHAR(32) NOT NULL,
  actor VARCHAR(64) NOT NULL,
  created_at VARCHAR(64) NOT NULL,
  INDEX idx_appointment_events_appointment (appointment_id, created_at)
);
CREATE TABLE IF NOT EXISTS followups (
  id VARCHAR(64) PRIMARY KEY,
  patient_id VARCHAR(64) NOT NULL,
  patient_name VARCHAR(64) NOT NULL,
  summary VARCHAR(255) NOT NULL,
  due_at VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  created_at VARCHAR(64) NOT NULL,
  updated_at VARCHAR(64) NOT NULL,
  INDEX idx_followups_status_due (status, due_at)
);

INSERT IGNORE INTO departments (id,name) VALUES
 ('dep-general','全科门诊'),('dep-derma','皮肤科'),('dep-rehab','康复理疗'),('dep-nutrition','营养咨询');
INSERT IGNORE INTO doctors (id,name,department,status,today_count) VALUES
 ('doc-01','林负责人','全科门诊','出诊中',18),('doc-02','沈负责人','皮肤科','出诊中',16),
 ('doc-03','赵负责人','康复理疗','出诊中',12),('doc-04','周负责人','营养咨询','休息中',10),
 ('doc-05','陈负责人','全科门诊','出诊中',14),('doc-06','王负责人','皮肤科','出诊中',16);
INSERT IGNORE INTO patients (id,name,phone,last_visit,created_at) VALUES
 ('PT-001','演示客户01','13800000001','2026-07-15','2026-07-01'),('PT-002','演示客户02','13800000002','2026-07-15','2026-07-01'),
 ('PT-003','演示客户03','13800000003','2026-07-14','2026-07-01'),('PT-004','演示客户04','13800000004','2026-07-14','2026-07-01'),
 ('PT-005','演示客户05','13800000005','2026-07-13','2026-07-01'),('PT-006','演示客户06','13800000006','2026-07-13','2026-07-01'),
 ('PT-007','演示客户07','13800000007','2026-07-12','2026-07-01'),('PT-008','演示客户08','13800000008','2026-07-12','2026-07-01'),
 ('PT-009','演示客户09','13800000009','2026-07-11','2026-07-01'),('PT-010','演示客户10','13800000010','2026-07-11','2026-07-01'),
 ('PT-011','演示客户11','13800000011','2026-07-10','2026-07-01'),('PT-012','演示客户12','13800000012','2026-07-10','2026-07-01'),
 ('PT-013','演示客户13','13800000013','2026-07-09','2026-07-01'),('PT-014','演示客户14','13800000014','2026-07-09','2026-07-01'),
 ('PT-015','演示客户15','13800000015','2026-07-08','2026-07-01'),('PT-016','演示客户16','13800000016','2026-07-08','2026-07-01'),
 ('PT-017','演示客户17','13800000017','2026-07-07','2026-07-01'),('PT-018','演示客户18','13800000018','2026-07-07','2026-07-01'),
 ('PT-019','演示客户19','13800000019','2026-07-06','2026-07-01'),('PT-020','演示客户20','13800000020','2026-07-06','2026-07-01'),
 ('PT-021','演示客户21','13800000021','2026-07-05','2026-07-01'),('PT-022','演示客户22','13800000022','2026-07-05','2026-07-01'),
 ('PT-023','演示客户23','13800000023','2026-07-04','2026-07-01'),('PT-024','演示客户24','13800000024','2026-07-04','2026-07-01'),
 ('PT-025','演示客户25','13800000025','2026-07-03','2026-07-01'),('PT-026','演示客户26','13800000026','2026-07-03','2026-07-01'),
 ('PT-027','演示客户27','13800000027','2026-07-02','2026-07-01'),('PT-028','演示客户28','13800000028','2026-07-02','2026-07-01'),
 ('PT-029','演示客户29','13800000029','2026-07-01','2026-07-01'),('PT-030','演示客户30','13800000030','2026-07-01','2026-07-01');
INSERT IGNORE INTO appointments (id,patient_id,patient_name,department,doctor,scheduled_at,status,created_at,updated_at) VALUES
 ('AP-0716-081','PT-001','演示客户01','全科门诊','林负责人','2026-07-16T08:00:00+08:00','已完成','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('AP-0716-082','PT-002','演示客户02','皮肤科','沈负责人','2026-07-16T09:00:00+08:00','处理中','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('AP-0716-083','PT-003','演示客户03','康复理疗','赵负责人','2026-07-16T10:00:00+08:00','候诊中','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('AP-0716-084','PT-004','演示客户04','营养咨询','周负责人','2026-07-16T11:00:00+08:00','已确认','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('AP-0716-085','PT-005','演示客户05','全科门诊','陈负责人','2026-07-16T12:00:00+08:00','待确认','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('AP-0716-086','PT-006','演示客户06','皮肤科','王负责人','2026-07-16T13:00:00+08:00','已完成','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('AP-0716-087','PT-007','演示客户07','康复理疗','赵负责人','2026-07-16T14:00:00+08:00','处理中','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('AP-0716-088','PT-008','演示客户08','营养咨询','周负责人','2026-07-16T15:00:00+08:00','候诊中','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('AP-0716-089','PT-009','演示客户09','全科门诊','林负责人','2026-07-16T16:00:00+08:00','已确认','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('AP-0716-090','PT-010','演示客户10','皮肤科','沈负责人','2026-07-16T17:00:00+08:00','待确认','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('AP-0716-091','PT-011','演示客户11','康复理疗','赵负责人','2026-07-16T08:30:00+08:00','已完成','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('AP-0716-092','PT-012','演示客户12','营养咨询','周负责人','2026-07-16T09:30:00+08:00','候诊中','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z');
INSERT IGNORE INTO followups (id,patient_id,patient_name,summary,due_at,status,created_at,updated_at) VALUES
 ('FW-0716-001','PT-001','演示客户01','复诊提醒与满意度回访','2026-07-17','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('FW-0716-002','PT-002','演示客户02','术后回访','2026-07-17','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('FW-0716-003','PT-003','演示客户03','康复计划提醒','2026-07-18','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('FW-0716-004','PT-004','演示客户04','饮食建议回访','2026-07-18','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('FW-0716-005','PT-005','演示客户05','复诊提醒','2026-07-19','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('FW-0716-006','PT-006','演示客户06','满意度回访','2026-07-19','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('FW-0716-007','PT-007','演示客户07','康复计划提醒','2026-07-20','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('FW-0716-008','PT-008','演示客户08','复诊提醒','2026-07-20','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('FW-0716-009','PT-009','演示客户09','术后回访','2026-07-21','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('FW-0716-010','PT-010','演示客户10','满意度回访','2026-07-21','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('FW-0716-011','PT-011','演示客户11','复诊提醒','2026-07-22','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('FW-0716-012','PT-012','演示客户12','康复计划提醒','2026-07-22','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z');

-- SupplyFlow 采购闭环（金额使用 DECIMAL，事件表支持审计与重放）
CREATE TABLE IF NOT EXISTS purchase_requests (id VARCHAR(64) PRIMARY KEY, requester VARCHAR(80) NOT NULL, department VARCHAR(80), reason VARCHAR(255), currency CHAR(3) NOT NULL DEFAULT 'CNY', estimated_amount DECIMAL(18,2) NOT NULL DEFAULT 0, status VARCHAR(20) NOT NULL, created_at DATETIME(6) NOT NULL, updated_at DATETIME(6) NOT NULL, KEY idx_purchase_requests_status(status), KEY idx_purchase_requests_created(created_at));
CREATE TABLE IF NOT EXISTS purchase_items (id VARCHAR(64) PRIMARY KEY, request_id VARCHAR(64) NOT NULL, sku VARCHAR(80), name VARCHAR(120) NOT NULL, quantity INT NOT NULL, unit VARCHAR(20), unit_price DECIMAL(18,2) NOT NULL DEFAULT 0, KEY idx_purchase_items_request(request_id));
CREATE TABLE IF NOT EXISTS supplier_quotes (id VARCHAR(64) PRIMARY KEY, request_id VARCHAR(64) NOT NULL, supplier VARCHAR(120) NOT NULL, amount DECIMAL(18,2) NOT NULL, delivery_days INT NOT NULL DEFAULT 0, status VARCHAR(20) NOT NULL, created_at DATETIME(6) NOT NULL, KEY idx_supplier_quotes_request(request_id));
CREATE TABLE IF NOT EXISTS purchase_orders (id VARCHAR(64) PRIMARY KEY, request_id VARCHAR(64) NOT NULL, supplier VARCHAR(120) NOT NULL, amount DECIMAL(18,2) NOT NULL, status VARCHAR(20) NOT NULL, created_at DATETIME(6) NOT NULL, UNIQUE KEY uk_purchase_orders_request(request_id));
CREATE TABLE IF NOT EXISTS goods_receipts (id VARCHAR(64) PRIMARY KEY, request_id VARCHAR(64) NOT NULL, order_id VARCHAR(64), quantity INT NOT NULL, warehouse VARCHAR(120), received_at DATETIME(6) NOT NULL, status VARCHAR(20) NOT NULL, KEY idx_goods_receipts_request(request_id));
CREATE TABLE IF NOT EXISTS quality_checks (id VARCHAR(64) PRIMARY KEY, request_id VARCHAR(64) NOT NULL, receipt_id VARCHAR(64), passed TINYINT(1) NOT NULL, note VARCHAR(255), checked_at DATETIME(6) NOT NULL, KEY idx_quality_checks_request(request_id));
CREATE TABLE IF NOT EXISTS stock_entries (id VARCHAR(64) PRIMARY KEY, request_id VARCHAR(64) NOT NULL, sku VARCHAR(80), quantity INT NOT NULL, warehouse VARCHAR(120), created_at DATETIME(6) NOT NULL, KEY idx_stock_entries_request(request_id));
CREATE TABLE IF NOT EXISTS purchase_events (id VARCHAR(64) PRIMARY KEY, request_id VARCHAR(64) NOT NULL, from_status VARCHAR(20), to_status VARCHAR(20) NOT NULL, actor VARCHAR(80) NOT NULL, note VARCHAR(255), created_at DATETIME(6) NOT NULL, KEY idx_purchase_events_request_created(request_id,created_at));
