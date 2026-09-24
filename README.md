# Computer Booking & Management System (AUCC Project)

ระบบจองและจัดการเครื่องคอมพิวเตอร์ในห้องปฏิบัติการ ใช้ Go, MySQL, WebSocket และ React

> เอกสารนี้เป็นขั้นตอนสำหรับ local trial; ยังไม่ถือเป็น production deployment จนกว่าจะตั้งค่า TLS/โดเมน, สำรองและทดสอบ restore ฐานข้อมูล, และทดสอบกับเครื่อง agent จริง

---

## สถาปัตยกรรมระบบ (Architecture)

```
React frontend ── REST/WebSocket ── Go backend ── MySQL
                                      │
                                      └── WebSocket ── Go client agent
```

ยังไม่มี benchmark ยืนยันจำนวนเครื่อง, RAM หรือ latency ที่รองรับ

---

## เตรียมค่าตั้งต้น

คัดลอก `.env.example` เป็น `.env` แล้วกำหนดค่าฐานข้อมูล, `JWT_SECRET` (อย่างน้อย 32 bytes) และบัญชี admin เริ่มต้น (`SADMIN_USERNAME`/`SADMIN_PASSWORD`; รหัสผ่าน 12–72 bytes) ก่อนเริ่ม backend ตัวอย่างเช่น สุ่ม JWT secret ด้วย `python -c "import secrets; print(secrets.token_urlsafe(48))"` ห้าม commit `.env` หรือใช้ secret ตัวอย่างในระบบจริง

---

## ขั้นตอนการติดตั้งและเริ่มใช้งาน (Quick Start)

ข้อกำหนด: Go 1.26 ขึ้นไป, Node.js/npm, Python และ MySQL

### 1. เตรียมฐานข้อมูล

สำรองฐานข้อมูลก่อน แล้วติดตั้ง PyMySQL (`python -m pip install pymysql`) และรัน `python migrations/apply_migrations.py` จากโฟลเดอร์โปรเจกต์ สคริปต์อ่าน `.env`, สร้างตาราง migration ledger และใช้ migration ที่ยังไม่เคยบันทึก **อย่ารันครั้งแรกกับฐานข้อมูลที่มีข้อมูลสำคัญโดยไม่มี backup**

### 2. รัน Go backend สำหรับ local trial

ค่า `.env.example` ใช้ loopback และ `COOKIE_SECURE=false` สำหรับเครื่องเดียวเท่านั้น; backend ปฏิเสธ plain HTTP ที่ bind ออกเครือข่าย ตั้งค่าฐานข้อมูลและ secret จริงก่อนเริ่ม:

```bash
cd backend-go
go run cmd/server/main.go
```

ตรวจ readiness ที่ `http://localhost:8000/health/ready` ซึ่งต้องตอบสถานะพร้อมใช้งานก่อนเปิด frontend

### 3. รัน React frontend

คัดลอก `frontend/.env.example` เป็น `frontend/.env.local` สำหรับ local trial แล้ว:

```bash
cd frontend
npm ci
npm run dev
```
เปิดเบราว์เซอร์ที่: **`http://localhost:5173`**

### 4. เพิ่มเครื่องและรัน client agent

สร้างสถานีจากหน้า Admin แล้วใช้ secret ที่แสดงครั้งเดียวใน agent ของสถานีนั้น ห้ามแชร์ credential ข้ามเครื่องหรือส่งต่อ secret ผ่าน chat/log ไฟล์ `client-go/config.json` เป็น local configuration และต้องอยู่นอก version control:

```bash
cd client-go
go run main.go
```

บน Windows agent จะแสดง overlay เต็มจอเมื่อได้รับ `LOCK` และซ่อนเมื่อได้รับ `UNLOCK`; ต้องรันใน session ของผู้ใช้ที่ล็อกอินอยู่ ไม่ใช่ Windows service overlay นี้ไม่ใช่ secure lock ของ Windows และควรใช้เครื่องทดสอบหรือ kiosk policy ที่ควบคุมไว้

สำหรับ agent รุ่นเก่าที่ใช้ `AGENT_SECRET` กลาง ให้คง `ALLOW_LEGACY_AGENTS=true` เฉพาะช่วงย้ายเครื่อง; หมุน secret รายสถานีและเปลี่ยนเป็น `ALLOW_LEGACY_AGENTS=false` หลังย้ายครบ

## ก่อนทดลองกับเครื่องในเครือข่ายหรือผู้ใช้จริง

- ตั้ง HTTPS/WSS ให้ frontend, API และ agent; ใช้ origin allowlist ให้ตรงกับหน้าเว็บ และเชื่อถือเฉพาะ reverse proxy ที่ระบุใน `TRUSTED_PROXIES`.
- กำหนด `COOKIE_SECURE=true`, ตั้งค่าการสำรอง/กู้คืน MySQL และทดลอง restore บนฐานข้อมูลแยก.
- ทดลองจองพร้อมกัน, ยกเลิก/ต่อเวลา, หมด session, เครื่องหลุด/ต่อใหม่ และคำสั่งที่เหมาะกับแต่ละ role บน staging.
- มีผู้ดูแลติดตาม logs และวิธี rollback release. โดเมนและ TLS สำหรับการเข้าจากเครือข่ายยังต้องกำหนดก่อนเริ่ม pilot ระยะไกล.
