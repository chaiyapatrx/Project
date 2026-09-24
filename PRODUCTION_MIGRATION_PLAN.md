> **เอกสารเก็บถาวร:** ไฟล์นี้เป็นแผนออกแบบย้อนหลัง ไม่ใช่คู่มือ deploy ปัจจุบัน รายละเอียดบางส่วน เช่น จำนวนเครื่องที่รองรับ, kiosk, schema และ mock data อาจไม่ตรงกับโค้ดแล้ว ให้อ้างอิงขั้นตอนล่าสุดใน `README.md` และผลประเมินใน `PRODUCTION_READINESS_REVIEW.md` ก่อนดำเนินการจริง

# แผนงานยกระดับระบบสู่ระดับ Production (Go + MySQL + WebSocket)

เอกสารแผนงานทางวิศวกรรมสำหรับการปรับปรุงสถาปัตยกรรมระบบจองและจัดการเครื่องคอมพิวเตอร์ ย้าย Backend สู่ภาษา Go, สื่อสารผ่าน WebSocket แบบ Real-time, พัฒนา Client Agent ด้วย Go และเชื่อมต่อ Frontend (React 19) โดยใช้ฐานข้อมูล MySQL เดิม

---

## สรุปปัญหาเดิมและสถาปัตยกรรมเป้าหมาย

### ปัญหาของระบบเดิม (Current Bottlenecks)
1. **HTTP Polling ถี่:** Client ส่ง POST `/api/heartbeat` ทุก 5 วินาที และ Frontend ยิง GET วนซ้ำ เปลือง Network & CPU
2. **I/O คอขวดที่ MySQL:** Server บันทึกข้อมูล Heartbeat ลงตาราง `computers` ในดิสก์ทุก 5 วินาทีต่อเครื่อง
3. **Client Agent ใช้ Python:** กิน RAM 50–80 MB ต่อเครื่อง, แพ็กไฟล์ใหญ่ และไม่สามารถล็อกเครื่องหรือป้องกันการกดปิดโปรแกรมได้จริง
4. **Command แบบ Polling:** คำสั่ง Lock/Shutdown ต้องรอบันทึกลงตาราง `ComputerCommand` แล้วรอ Agent ดึงไปรัน มี Latency 5–10 วินาที
5. **หน้า Executive/Settings ใช้ Mock Data:** ขาด Aggregation API จริงสำหรับสถิติเชิงบริหาร

### สถาปัตยกรรมใหม่ (Target Production Architecture)

```
[ เครื่องลูกข่าย (50-200 เครื่อง) ]
  Go Client Agent (.exe เบื้องหลัง)
  ├── 1. WebSocket Client (Ping-Pong ทุก 15s)
  ├── 2. Command Listener (Lock, Unlock, Shutdown, Reboot)
  └── 3. Windows Kiosk & Lock Screen (Hook ดัก Win, TaskMgr, Alt+Tab)
            │
            ▼ (WSS / WebSocket over TLS)
[ Go Backend Server ]
  ├── Router & Middlewares (Fiber หรือ Gin)
  ├── In-Memory Hub & State Machine (เก็บสถานะ Online/Busy/Locked บน RAM)
  ├── Session Timer Worker (Goroutine ตรวจสอบหมดเวลาการจองอัตโนมัติ)
  └── MySQL Pool (sqlx หรือ GORM)
            │ (บันทึกเฉพาะ Event สำคัญ: Login, จอง, เริ่ม/จบคลาส, Audit Log)
            ▼
[ MySQL 8.0 Database ]
            ▲
            │ (WebSocket + REST API)
[ Web Frontend (React 19) ]
  Staff, Admin, Executive, Student
```

---

## Phase 1: จัดระเบียบฐานข้อมูล MySQL และปรับ Schema ให้พร้อม Production

### วัตถุประสงค์
หยุดการบันทึกสถานะชั่วคราว (Transient State) ลงดิสก์, เพิ่มตารางสำหรับการตั้งค่าจริง, และกำหนด Migration ที่ตรวจสอบย้อนหลังได้

### รายการงานที่ต้องทำ
1. **ออกแบบระบบ Database Migration:**
   - ใช้เครื่องมือ `golang-migrate` จัดการไฟล์ `.sql` แบ่งเป็น `up.sql` และ `down.sql`
   - เลิกใช้ `models.Base.metadata.create_all()` แบบ Python เดิม

2. **ปรับปรุง Schema ตารางเดิม:**
   - ตาราง `computers`:
     - เก็บเฉพาะ Master Data: `id`, `name`, `hwid`, `ip_address`, `mac_address`, `is_active`, `created_at`
     - ตัดคอลัมน์ `last_heartbeat` ออกจากดิสก์ (ย้ายไปอยู่บน RAM ของ Go Server)
     - เพิ่มดัชนี (Index): `UNIQUE KEY idx_computers_hwid (hwid)`, `UNIQUE KEY idx_computers_name (name)`
   - ตาราง `users`:
     - ปรับประเภทของคอลัมน์ `role` เป็น ENUM: `'admin'`, `'staff'`, `'executive'`, `'student'`
     - เพิ่มคอลัมน์ `is_active BOOLEAN DEFAULT TRUE`
     - เพิ่มดัชนี `idx_users_username`

3. **ตัดตารางที่ไม่จำเป็น:**
   - **ลบตาราง `computer_commands`:** คำสั่งทั้งหมดจะส่งตรงผ่าน WebSocket In-Memory Connection หากต้องการเก็บประวัติ ให้เปลี่ยนเป็นตาราง `audit_logs` สำหรับบันทึกว่าใครสั่งคำสั่งอะไร เมื่อไหร่ สำเร็จหรือไม่

4. **สร้างตารางใหม่ที่จำเป็นสำหรับการใช้งานจริง:**
   - **ตาราง `system_settings` (Key-Value):**
     - Schema: `setting_key VARCHAR(50) PRIMARY KEY`, `setting_value TEXT NOT NULL`, `description VARCHAR(255)`, `updated_at TIMESTAMP`
     - ค่าเริ่มต้น: `session_duration_minutes`, `maintenance_mode`, `idle_timeout_minutes`, `allow_guest_login`
   - **ตาราง `audit_logs`:**
     - Schema: `id`, `user_id`, `action`, `target_machine`, `ip_address`, `details`, `created_at`

---

## Phase 2: พัฒนา Go Backend Core & WebSocket State Machine

### วัตถุประสงค์
แทนที่ FastAPI ด้วย Go Server ที่กิน RAM ไม่เกิน 30 MB, ตอบสนองระดับ Microseconds, และคุม State ผ่านหน่วยความจำ

### รายการงานที่ต้องทำ
1. **จัดโครงสร้างโปรเจกต์ (Clean Architecture):**
   ```
   backend-go/
   ├── cmd/server/main.go
   ├── internal/
   │   ├── config/          # โหลด .env
   │   ├── database/        # MySQL connection pool + sqlx/GORM
   │   ├── auth/            # JWT Token creation, validation, bcrypt
   │   ├── hub/             # WebSocket Connection Hub & State Machine
   │   ├── models/          # Go Structs สำหรับ Database
   │   ├── repository/      # Query ฐานข้อมูล MySQL
   │   ├── service/         # Business Logic (จองเครื่อง, คืนสิทธิ์)
   │   ├── handler/         # HTTP & WebSocket Handlers
   │   └── worker/          # Background Goroutines (Session Countdown)
   └── migrations/
   ```

2. **ตั้งค่า MySQL Connection Pool ให้เหมาะสม:**
   - `SetMaxOpenConns(50)`
   - `SetMaxIdleConns(25)`
   - `SetConnMaxLifetime(5 * time.Minute)`

3. **สร้าง In-Memory State Machine (Hub):**
   - สร้าง Struct เก็บสถานะเครื่องใน RAM โดยมี `sync.RWMutex` ป้องกัน Race Condition:
     ```go
     type MachineState struct {
         ID            uint
         Name          string
         HWID          string
         IPAddress     string
         Status        string // available, in_use, locked, offline
         IsOnline      bool
         CurrentUserID *uint
         SessionEndsAt *time.Time
         Conn          *websocket.Conn
     }
     ```
   - ฝั่ง Web Client (Admin/Staff) ลงทะเบียนรับ Broadcast เมื่อ `MachineState` มีการเปลี่ยนแปลง

4. **พอร์ต REST API เดิมมาเป็น Go:**
   - Auth: `/api/auth/login`, `/api/auth/register`, `/api/auth/me`
   - Computers: `/api/computers` (CRUD)
   - Bookings: `/api/bookings` (สร้างการจอง, ยกเลิกการจอง)
   - Settings: `/api/admin/settings` (ดึงและแก้ไขการตั้งค่าระบบ)

5. **ระบบ Background Worker (Goroutine Ticker):**
   - รัน Ticker ทุก 1 วินาที:
     - ตรวจสอบเครื่องที่ `SessionEndsAt <= time.Now()`
     - ยิงคำสั่ง `{"action": "LOCK", "reason": "timeout"}` ผ่าน WebSocket ไปยังเครื่องลูกข่ายทันที
     - บันทึกประวัติลงตาราง `usage_logs`
     - Broadcast สถานะอัปเดตให้หน้า Dashboard

---

## Phase 3: พัฒนา Go Client Agent และระบบความปลอดภัยหน้าเครื่อง

### วัตถุประสงค์
แทนที่ Python Script ด้วย Go Binary ขนาดเล็ก (~8 MB, RAM < 10 MB) ที่ทำงานเบื้องหลัง ควบคุมหน้าจอได้จริง ป้องกันการหลบเลี่ยง

### รายการงานที่ต้องทำ
1. **การพิสูจน์ตัวตนเครื่อง (Machine Authentication):**
   - ดึง Hardware ID อัตโนมัติ: ดึงจาก BIOS/Motherboard UUID ร่วมกับ MAC Address แรกที่ไม่ใช่ Virtual
   - อ่าน `agent_config.json` (เก็บ `server_url` และ `machine_token`)
   - ยืนยันสิทธิ์กับ Server ผ่าน WSS Handshake Header

2. **ระบบการเชื่อมต่อแบบทนทาน (Resilient WebSocket Connection):**
   - ส่ง Ping Frame ทุก 15 วินาที (หาก Server ไม่ตอบ Pong ใน 5 วินาที ให้ตัด Connection ทันที)
   - กลไก Auto-reconnect: ทำงานแบบ Exponential Backoff (1s -> 2s -> 4s -> สูงสุด 30s) ไม่ทำให้เครื่องค้าง

3. **การรับคำสั่งจาก Server (Command Listener):**
   - รองรับ JSON Payload:
     - `LOCK`: เปิดหน้าต่างล็อกหน้าจอ
     - `UNLOCK`: ปิดหน้าต่างล็อกหน้าจอและเปิดให้ใช้งาน
     - `REBOOT`: เรียกคำสั่ง `shutdown /r /t 0`
     - `SHUTDOWN`: เรียกคำสั่ง `shutdown /s /t 0`
     - `NOTIFICATION`: แสดงแจ้งเตือนเตือนเวลาใกล้หมดบน Taskbar

4. **ระบบ Kiosk Lock Screen (ความปลอดภัยระดับ OS):**
   - **หน้าต่างล็อก:** สร้าง GUI ด้วยไลบรารี Go GUI ขนาดเบา (เช่น Walk หรือ Fyne) ให้เป็นหน้าต่าง Topmost, Fullscreen ทับหน้าจอทั้งหมด
   - **ดักปุ่มคีย์บอร์ด (Low-Level Windows Hook):**
     - ใช้ `SetWindowsHookExW` (`WH_KEYBOARD_LL`) ดักปุ่ม:
       - `VK_LWIN`, `VK_RWIN` (ปุ่ม Windows)
       - `Alt + Tab`, `Alt + Esc`, `Alt + F4`
       - `Ctrl + Esc`
   - **ปิดการใช้งาน Task Manager:**
     - แก้ไข Registry ค่า `DisableTaskMgr = 1` ในคีย์ `HKCU\Software\Microsoft\Windows\CurrentVersion\Policies\System` ขณะที่ล็อกอยู่
     - ปลดกลับเป็น `0` เมื่อได้รับคำสั่ง UNLOCK

---

## Phase 4: ปรับปรุง Frontend (React 19) และยกเลิก Mock Data

### วัตถุประสงค์
ยกเลิกการ Polling ทุกหน้า เปลี่ยนมารับข้อมูล Real-time ผ่าน WebSocket และต่อ API จริงให้ครบทุกหน้า

### รายการงานที่ต้องทำ
1. **สร้าง Global WebSocket Hook (`useSocketMonitor`):**
   - เชื่อมต่อ WebSocket 1 เส้นต่อ 1 Browser Session
   - ดัก Event:
     - `MACHINE_STATUS_UPDATED`: อัปเดต State เครื่องที่เปลี่ยนสถานะ (ไม่ทำให้เครื่องอื่น Re-render)
     - `SESSION_COUNTDOWN`: แจ้งเตือนเวลานับถอยหลัง
     - `ALERT_TRIGGERED`: แจ้งเตือนฉุกเฉินให้ Staff/Admin

2. **ปรับปรุงหน้านักเรียน/ผู้ใช้งาน (`StudentDashboard.jsx`):**
   - หน้าจอแสดงสิทธิ์การจอง และเวลานับถอยหลังจริงที่ Sync ตรงกับ Server
   - ปุ่ม "เริ่มใช้งาน (Unlock เครื่อง)" สั่งผ่าน API และปลดล็อกหน้าจอเครื่องจริงทันที

3. **ปรับปรุงหน้า Staff & Admin:**
   - `StaffSessionControl.jsx`: กดปุ่มสั่ง Lock / Unlock / Force Logout แล้วเห็นผลบนเครื่องจริงทันที (< 50ms)
   - `AdminSettings.jsx`: ลบฟังก์ชัน Simulate ทิ้ง เชื่อมต่อกับ `/api/admin/settings` จริง

4. **ยกเลิก Mock Data ในหน้า Executive:**
   - `ExecOverview.jsx` & `ExecReports.jsx`:
     - สร้าง Endpoint บน Go Server: `/api/admin/analytics/summary`
     - ใช้ SQL Aggregation คำนวณจริง:
       - อัตราการใช้งานรายชั่วโมง (Hourly Utilization)
       - ภาควิชาที่ใช้งานสูงสุด (Top Departments)
       - ระยะเวลาการใช้งานเฉลี่ย (Average Session Duration)
     - นำข้อมูลจริงมา Render ลงชาร์ต

---

## Phase 5: ผูก Business Logic และ Flow การทำงานจริง

### Flow การทำงานแบบบูรณาการ (End-to-End Workflow)

1. **การบูตเครื่องลูกข่าย (Machine Boot):**
   - Client Agent รันขึ้นมาเป็น Windows Service
   - หน้าจอล็อกตัวเองทันที (Kiosk Lock) จนกว่าจะมีคำสั่งจาก Server
   - Agent ยิง WebSocket หา Server -> Server อัปเดตสถานะใน RAM เป็น `Available` (พร้อมใช้งาน) -> หน้าเว็บ Staff ขึ้นไฟเขียว

2. **การจองและการเข้าใช้งาน (Booking & Activation):**
   - นักเรียนกดจองเครื่อง `COM-05` บนมือถือหรือหน้าเว็บ
   - Server ตรวจสอบสิทธิ์ใน MySQL -> บันทึกสถานะ In-Memory เป็น `Booked`
   - เมื่อนักเรียนมาถึงหน้าเครื่องและกดยืนยันตัวตน (หรือ Staff กดเปิดให้):
     - Server ส่งคำสั่ง `UNLOCK` ผ่าน WebSocket ไปยัง Agent ของ `COM-05`
     - Agent ปิดหน้าต่าง Lock Screen คืนสิทธิ์การใช้งาน
     - Background Worker เริ่มนับเวลาถอยหลัง

3. **การหมดเวลาและตัดสิทธิ์ (Expiration & Teardown):**
   - เมื่อเหลือ 5 นาที: Server ส่ง `NOTIFICATION` ไปแสดงข้อความบนหน้าจอเครื่องลูก
   - เมื่อหมดเวลา (0 นาที):
     - Server ส่ง `LOCK` ไปที่ Agent
     - Agent เปิด Kiosk Lock ทับหน้าจอทันที
     - Server บันทึกข้อมูลสรุป (เริ่มกี่โมง, เลิกกี่โมง, กี่นาที) ลงตาราง `usage_logs` ใน MySQL
     - Server คืนสถานะเครื่องเป็น `Available`

---

## Phase 6: Production Hardening, Security & Deployment

### รายการตรวจสอบความปลอดภัยและความพร้อม (Production Checklist)

1. **Security & Network:**
   - บังคับใช้ HTTPS และ WSS (TLS 1.3) ผ่าน Reverse Proxy (Nginx หรือ Caddy)
   - ปิด Port 8000/8080 ไม่ให้เข้าถึงจากภายนอกโดยตรง เปิดเฉพาะ 443
   - แยก Network Subnet สำหรับเครื่องลูกข่ายและเครื่อง Server

2. **Graceful Shutdown (Go Server):**
   - ดักจับสัญญาณ `SIGINT` และ `SIGTERM`
   - เมื่อ Server จะปิด ให้ส่งสัญญาณแจ้งเครื่องลูกข่ายและหน้าเว็บก่อนปิด Socket Cleanly

3. **Client Agent Service Management:**
   - ใช้เครื่องมือลงทะเบียนเป็น Windows Service แท้จริง (เช่น NSSM หรือ Go `golang.org/x/sys/windows/svc`)
   - ตั้งค่า Service Recovery ให้ Restart อัตโนมัติทันทีหาก Process ถูกสั่ง End Process

4. **Logging & Monitoring:**
   - ใช้ Structured Logger บน Go (เช่น `uber-go/zap` หรือ `rs/zerolog`)
   - หมุนเวียน Log (Log Rotation) ไม่ให้กินพื้นที่ดิสก์ Server

---

## สรุปการแบ่ง Phase และระยะเวลาโดยประมาณ

| Phase | ชื่องาน | ระยะเวลาประเมิน | ความเสี่ยง |
| :--- | :--- | :---: | :---: |
| **Phase 1** | ปรับ Database Schema & Migration (MySQL) | 1–2 วัน | ต่ำ |
| **Phase 2** | พัฒนา Go Backend Core & WebSocket Hub | 4–6 วัน | ปานกลาง |
| **Phase 3** | พัฒนา Go Client Agent & Windows Lock Hook | 4–6 วัน | สูง (ต้องเทสกับ OS หลากหลาย) |
| **Phase 4** | ปรับ Frontend React 19 เลิก Mock ต่อ WebSocket | 2–3 วัน | ต่ำ |
| **Phase 5** | เชื่อม Business Logic & ระบบนับเวลาอัตโนมัติ | 2–3 วัน | ปานกลาง |
| **Phase 6** | Hardening, Security & ติดตั้งระบบจริง | 2–3 วัน | ปานกลาง |
