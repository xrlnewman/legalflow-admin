-- LegalFlow synthetic operational data only. Never load real medical records here.
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

CREATE TABLE IF NOT EXISTS matters (
  id VARCHAR(64) PRIMARY KEY,
  subject_alias VARCHAR(128) NOT NULL,
  case_type VARCHAR(64) NOT NULL,
  priority VARCHAR(16) NOT NULL,
  deadline VARCHAR(32) NOT NULL,
  assignee VARCHAR(64) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL,
  closure_result VARCHAR(255) NOT NULL DEFAULT '',
  created_at VARCHAR(64) NOT NULL,
  updated_at VARCHAR(64) NOT NULL,
  INDEX idx_matters_status_deadline (status, deadline),
  INDEX idx_matters_assignee_deadline (assignee, deadline)
);
CREATE TABLE IF NOT EXISTS matter_tasks (
  id VARCHAR(64) PRIMARY KEY,
  matter_id VARCHAR(64) NOT NULL,
  title VARCHAR(128) NOT NULL,
  assignee VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  created_at VARCHAR(64) NOT NULL,
  updated_at VARCHAR(64) NOT NULL,
  UNIQUE KEY uq_matter_task (matter_id),
  CONSTRAINT fk_matter_tasks_matter FOREIGN KEY (matter_id) REFERENCES matters(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS matter_documents (
  id VARCHAR(64) PRIMARY KEY,
  matter_id VARCHAR(64) NOT NULL,
  name VARCHAR(255) NOT NULL,
  kind VARCHAR(64) NOT NULL,
  checksum VARCHAR(255) NOT NULL,
  created_at VARCHAR(64) NOT NULL,
  created_by VARCHAR(64) NOT NULL,
  UNIQUE KEY uq_matter_document_checksum (matter_id, checksum),
  INDEX idx_matter_documents_created (matter_id, created_at),
  CONSTRAINT fk_matter_documents_matter FOREIGN KEY (matter_id) REFERENCES matters(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS matter_events (
  id VARCHAR(64) PRIMARY KEY,
  matter_id VARCHAR(64) NOT NULL,
  action VARCHAR(64) NOT NULL,
  from_status VARCHAR(32) NOT NULL DEFAULT '',
  to_status VARCHAR(32) NOT NULL DEFAULT '',
  actor VARCHAR(64) NOT NULL,
  detail VARCHAR(255) NOT NULL DEFAULT '',
  created_at VARCHAR(64) NOT NULL,
  INDEX idx_matter_events_timeline (matter_id, created_at, id),
  CONSTRAINT fk_matter_events_matter FOREIGN KEY (matter_id) REFERENCES matters(id) ON DELETE CASCADE
);

INSERT IGNORE INTO matters (id,subject_alias,case_type,priority,deadline,assignee,status,closure_result,created_at,updated_at) VALUES
 ('LF-0720-001','演示案卷-001','合同审查','高','2026-07-21','林律师','已立案','','2026-07-16T01:00:00Z','2026-07-16T01:00:00Z'),
 ('LF-0720-002','演示案卷-002','劳动争议','中','2026-07-22','沈律师','协同中','','2026-07-16T01:00:00Z','2026-07-16T02:00:00Z'),
 ('LF-0720-003','演示案卷-003','知识产权','高','2026-07-23','赵律师','待结案','','2026-07-16T01:00:00Z','2026-07-16T03:00:00Z'),
 ('LF-0720-004','演示案卷-004','合同审查','低','2026-07-24','','待委托','','2026-07-16T01:00:00Z','2026-07-16T01:00:00Z'),
 ('LF-0720-005','演示案卷-005','公司治理','中','2026-07-25','林律师','已结案','材料已归档，待客户确认','2026-07-16T01:00:00Z','2026-07-16T04:00:00Z');

INSERT IGNORE INTO departments (id,name) VALUES
 ('practice-contract','合同纠纷'),('practice-labor','劳动争议'),('practice-ip','知识产权'),('practice-corp','公司治理');
INSERT IGNORE INTO doctors (id,name,department,status,today_count) VALUES
 ('doc-01','林律师','合同纠纷','办理中',18),('doc-02','沈律师','劳动争议','办理中',16),
 ('doc-03','赵律师','知识产权','办理中',12),('doc-04','周律师','公司治理','休息中',10),
 ('doc-05','陈律师','合同纠纷','办理中',14),('doc-06','王律师','劳动争议','办理中',16);
INSERT IGNORE INTO patients (id,name,phone,last_visit,created_at) VALUES
 ('PT-001','演示当事人01','13800000001','2026-07-15','2026-07-01'),('PT-002','演示当事人02','13800000002','2026-07-15','2026-07-01'),
 ('PT-003','演示当事人03','13800000003','2026-07-14','2026-07-01'),('PT-004','演示当事人04','13800000004','2026-07-14','2026-07-01'),
 ('PT-005','演示当事人05','13800000005','2026-07-13','2026-07-01'),('PT-006','演示当事人06','13800000006','2026-07-13','2026-07-01'),
 ('PT-007','演示当事人07','13800000007','2026-07-12','2026-07-01'),('PT-008','演示当事人08','13800000008','2026-07-12','2026-07-01'),
 ('PT-009','演示当事人09','13800000009','2026-07-11','2026-07-01'),('PT-010','演示当事人10','13800000010','2026-07-11','2026-07-01'),
 ('PT-011','演示当事人11','13800000011','2026-07-10','2026-07-01'),('PT-012','演示当事人12','13800000012','2026-07-10','2026-07-01'),
 ('PT-013','演示当事人13','13800000013','2026-07-09','2026-07-01'),('PT-014','演示当事人14','13800000014','2026-07-09','2026-07-01'),
 ('PT-015','演示当事人15','13800000015','2026-07-08','2026-07-01'),('PT-016','演示当事人16','13800000016','2026-07-08','2026-07-01'),
 ('PT-017','演示当事人17','13800000017','2026-07-07','2026-07-01'),('PT-018','演示当事人18','13800000018','2026-07-07','2026-07-01'),
 ('PT-019','演示当事人19','13800000019','2026-07-06','2026-07-01'),('PT-020','演示当事人20','13800000020','2026-07-06','2026-07-01'),
 ('PT-021','演示当事人21','13800000021','2026-07-05','2026-07-01'),('PT-022','演示当事人22','13800000022','2026-07-05','2026-07-01'),
 ('PT-023','演示当事人23','13800000023','2026-07-04','2026-07-01'),('PT-024','演示当事人24','13800000024','2026-07-04','2026-07-01'),
 ('PT-025','演示当事人25','13800000025','2026-07-03','2026-07-01'),('PT-026','演示当事人26','13800000026','2026-07-03','2026-07-01'),
 ('PT-027','演示当事人27','13800000027','2026-07-02','2026-07-01'),('PT-028','演示当事人28','13800000028','2026-07-02','2026-07-01'),
 ('PT-029','演示当事人29','13800000029','2026-07-01','2026-07-01'),('PT-030','演示当事人30','13800000030','2026-07-01','2026-07-01');
INSERT IGNORE INTO appointments (id,patient_id,patient_name,department,doctor,scheduled_at,status,created_at,updated_at) VALUES
 ('LG-0716-081','PT-001','演示当事人01','合同纠纷','林律师','2026-07-16T08:00:00+08:00','已结案','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('LG-0716-082','PT-002','演示当事人02','劳动争议','沈律师','2026-07-16T09:00:00+08:00','办理中','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('LG-0716-083','PT-003','演示当事人03','知识产权','赵律师','2026-07-16T10:00:00+08:00','待办理','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('LG-0716-084','PT-004','演示当事人04','公司治理','周律师','2026-07-16T11:00:00+08:00','已立案','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('LG-0716-085','PT-005','演示当事人05','合同纠纷','陈律师','2026-07-16T12:00:00+08:00','待立案','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('LG-0716-086','PT-006','演示当事人06','劳动争议','王律师','2026-07-16T13:00:00+08:00','已结案','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('LG-0716-087','PT-007','演示当事人07','知识产权','赵律师','2026-07-16T14:00:00+08:00','办理中','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('LG-0716-088','PT-008','演示当事人08','公司治理','周律师','2026-07-16T15:00:00+08:00','待办理','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('LG-0716-089','PT-009','演示当事人09','合同纠纷','林律师','2026-07-16T16:00:00+08:00','已立案','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('LG-0716-090','PT-010','演示当事人10','劳动争议','沈律师','2026-07-16T17:00:00+08:00','待立案','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('LG-0716-091','PT-011','演示当事人11','知识产权','赵律师','2026-07-16T08:30:00+08:00','已结案','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z'),
 ('LG-0716-092','PT-012','演示当事人12','公司治理','周律师','2026-07-16T09:30:00+08:00','待办理','2026-07-16T00:00:00Z','2026-07-16T01:00:00Z');
INSERT IGNORE INTO followups (id,patient_id,patient_name,summary,due_at,status,created_at,updated_at) VALUES
 ('LG-0716-001','PT-001','演示当事人01','证据清单与举证期限','2026-07-17','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('LG-0716-002','PT-002','演示当事人02','庭审材料校对','2026-07-17','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('LG-0716-003','PT-003','演示当事人03','合同合规意见归档','2026-07-18','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('LG-0716-004','PT-004','演示当事人04','结案文书归档','2026-07-18','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('LG-0716-005','PT-005','演示当事人05','证据交换提醒','2026-07-19','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('LG-0716-006','PT-006','演示当事人06','诉讼节点复核','2026-07-19','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('LG-0716-007','PT-007','演示当事人07','合同条款复核','2026-07-20','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('LG-0716-008','PT-008','演示当事人08','庭前清单提醒','2026-07-20','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('LG-0716-009','PT-009','演示当事人09','证据目录归档','2026-07-21','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('LG-0716-010','PT-010','演示当事人10','知识产权材料核对','2026-07-21','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('LG-0716-011','PT-011','演示当事人11','公司治理文件归档','2026-07-22','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z'),
 ('LG-0716-012','PT-012','演示当事人12','劳动争议材料提醒','2026-07-22','待完成','2026-07-16T00:00:00Z','2026-07-16T00:00:00Z');
