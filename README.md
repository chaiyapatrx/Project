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

เมื่อ `DB_HOST` อยู่คนละเครื่อง backend และ migration จะตรวจ TLS certificate ของ MySQL รวมถึงชื่อ host ก่อนเข้าสู่ระบบ หาก CA ของฐานข้อมูลไม่อยู่ใน trust store ของเครื่อง ให้ขอไฟล์ CA จากผู้ดูแลฐานข้อมูลแล้วตั้ง `DB_TLS_CA_FILE` เป็นพาธไฟล์นั้น ห้ามใช้ certificate ที่ดึงจากการเชื่อมต่อที่ยังไม่ผ่านการตรวจสอบมาเป็น CA เอง

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

### 4. เพิ่มเครื่องและติดตั้ง client agent

ติดตั้ง agent บนเครื่อง client ก่อน ตัวติดตั้งจะส่งคำขอลงทะเบียนโดยใช้ชื่อคอมพิวเตอร์และ secret ที่สร้างในเครื่อง จากนั้นผู้ดูแลเปิดหน้า Admin → Computer Management ตรวจชื่อเครื่องและ HWID แล้วกด Approve เครื่องจึงจะเชื่อมต่อและเปิดให้จองได้ ไม่ต้องคัดลอก station secret

บนเครื่องที่ใช้ **สร้างตัวติดตั้ง** ให้ติดตั้ง Go และ Inno Setup แล้วรัน `client-go/build_installer.bat` ผลลัพธ์คือ `client-go/dist/AUCCAgentSetup-1.0.0.exe` เครื่อง client ไม่ต้องติดตั้ง Go หรือ Inno Setup

ถ้าทราบ URL ของ backend แล้ว ให้ตั้ง `AUCC_SERVER_URL` เป็น `wss://<server>/api/ws/agent` ก่อนสร้างตัวติดตั้ง ตัวติดตั้งชุดนั้นจะใส่ URL ให้ทุกเครื่องโดยอัตโนมัติ และข้ามหน้าถาม URL; เมื่อยังไม่กำหนดโดเมน ตัวติดตั้งจะถาม URL ตอนติดตั้งตามปกติ

บนเครื่อง client ให้เข้า Windows ด้วยบัญชีที่จะใช้งาน agent แล้วเปิด Setup:

1. เลือกที่ติดตั้ง ค่าเริ่มต้นคือ `%LOCALAPPDATA%\Programs\AUCC Agent` (ปกติอยู่บนไดรฟ์ C) หรือเลือกพาธอื่นที่บัญชีนี้เขียนได้
2. กรอก server URL อย่างเดียว; ถ้ารัน backend ในเครื่อง client เดียวกันใช้ `ws://localhost:8000/api/ws/agent` ถ้า backend อยู่คนละเครื่องให้ใช้ `wss://` ที่ Windows เชื่อถือ certificate
3. ตัวเลือกเริ่ม agent เมื่อ sign in เข้า Windows ถูกเลือกไว้แล้ว Setup จะเก็บ config ไว้ที่ `%LOCALAPPDATA%\AUCC Agent\config.json` ของบัญชีนี้ และสร้าง shortcut ใน Startup; ไม่ต้องคัดลอก config ไปไว้ข้าง `.exe`

Setup ส่งคำขอลงทะเบียนทันทีโดยไม่เปิดหน้า overlay; ถ้า server ยังไม่พร้อม agent จะลองใหม่เมื่อเริ่มทำงาน Setup ไม่เปิด agent ทันทีโดยปริยาย เพราะ agent จะล็อกหน้าจอเมื่อเริ่ม ผู้ติดตั้งเลือก `Start AUCC Agent now` ในหน้าสุดท้าย หรือ sign out แล้ว sign in อีกครั้ง การอัปเดตบนบัญชีเดิมจะคง config เดิมไว้ ตัวถอนติดตั้งจะลบ config ของบัญชีนั้นด้วย

### อัปเดต Agent จากเว็บ

ติดตั้ง `AUCCAgentSetup-1.0.0.exe` หนึ่งครั้งต่อเครื่องเพื่อให้มี `AUCCUpdater.exe` เครื่องที่ใช้ Agent รุ่นก่อนหน้านี้ต้องติดตั้งรุ่นนี้ด้วยมือครั้งเดียว จากนั้นอัปเดต Agent รุ่นถัดไปผ่านเว็บได้:

1. เปลี่ยนเลขรุ่นใน `client-go/VERSION` แล้วรัน `client-go/build_installer.bat` บนเครื่องสร้างรุ่นใหม่
2. Admin เปิด **Computer Management → Agent updates** แล้วอัปโหลด `client-go/dist/AUCCAgent.exe` พร้อมเลขรุ่นเดียวกับ `VERSION` (ไม่ใช่ไฟล์ Setup)
3. Admin หรือ Staff กด **Roll out** ให้รุ่นนั้น เครื่องออนไลน์ที่ไม่มี session ใช้งานเช็กภายในหนึ่งนาที เครื่องที่กำลังใช้งานจะรอจนว่าง เครื่องออฟไลน์เช็กเมื่อเชื่อมต่ออีกครั้ง หน้าเว็บแสดงรุ่นของแต่ละเครื่องและจำนวนที่อัปเดตแล้ว
4. หากพบปัญหา ให้กด **Pause** เพื่อหยุดเครื่องที่ยังไม่อัปเดต หรือกด **Roll out** ของรุ่นก่อนเพื่อย้อนกลับ เครื่องที่เปิดรุ่นใหม่ไปแล้วจะรับรุ่นก่อนเมื่อเช็กครั้งถัดไป

Agent ตรวจขนาดและ SHA-256 ของไฟล์ก่อนเปลี่ยน, ตรวจเลขรุ่นของ EXE และเก็บสำเนารุ่นเดิมไว้จนรุ่นใหม่เปิดผ่าน 15 วินาที หากเปิดไม่ผ่าน updater จะคืนรุ่นเดิมและบันทึก `update.log` ที่โฟลเดอร์ติดตั้ง พร้อมหยุดลองรุ่นที่ล้มเหลวซ้ำ ตัวโปรแกรมและตัว updater ต้องอยู่ในพาธที่บัญชี Windows นี้เขียนได้ ตั้ง `AGENT_RELEASE_DIR` เป็นโฟลเดอร์ถาวรบน server และสำรองโฟลเดอร์นี้พร้อมฐานข้อมูลก่อน deploy; ถ้ามี backend หลาย instance ให้ใช้ shared storage เดียวกัน อัปเดตผ่านเว็บนี้ครอบคลุมเฉพาะ Agent; backend/frontend ยัง deploy บน server ตามปกติ

บน Windows agent เริ่มต้นในสถานะล็อกระหว่างเชื่อมต่อ backend จากนั้นแสดงหน้าล็อกอินตามสถานะเครื่อง: ถ้าไม่มี booking ให้ใช้ชื่อผู้ใช้และรหัสผ่านของบัญชี AUCC; ถ้ามี booking ที่กำลังใช้งาน ให้กรอก Access Code 6 หลักของ booking สำหรับเครื่องนั้นเท่านั้น เมื่อยกเลิกหรือหมดเวลา ระบบล็อกเครื่องและกลับไปแสดงหน้าล็อกอินปกติ

overlay นี้ทำงานใน Windows session ที่ล็อกอินอยู่ จึงต้องเริ่ม agent ใน interactive session ไม่ใช่ Windows service และไม่ใช่หน้าล็อกอินก่อนเข้า Windows สำหรับการพัฒนาโดยไม่ใช้ตัวติดตั้งยังใช้ `client-go/build_agent.bat` และ [config.example.json](client-go/config.example.json) ข้าง `agent.exe` ได้

ใช้บัญชี Windows แบบจำกัดสิทธิ์สำหรับ agent; overlay นี้เป็นการบังหน้าจอระดับแอป ไม่ใช่ Windows secure lock และตัวติดตั้งไม่ตั้ง auto-logon ให้ VM

สำหรับ agent รุ่นเก่าที่ใช้ `AGENT_SECRET` กลาง ให้คง `ALLOW_LEGACY_AGENTS=true` เฉพาะช่วงย้ายเครื่อง; หมุน secret รายสถานีและเปลี่ยนเป็น `ALLOW_LEGACY_AGENTS=false` หลังย้ายครบ

## ก่อนทดลองกับเครื่องในเครือข่ายหรือผู้ใช้จริง

- ตั้ง HTTPS/WSS ให้ frontend, API และ agent; ใช้ origin allowlist ให้ตรงกับหน้าเว็บ และเชื่อถือเฉพาะ reverse proxy ที่ระบุใน `TRUSTED_PROXIES`.
- กำหนด `COOKIE_SECURE=true`, ตั้งค่าการสำรอง/กู้คืน MySQL และทดลอง restore บนฐานข้อมูลแยก.
- ทดลองจองพร้อมกัน, ยกเลิก/ต่อเวลา, หมด session, เครื่องหลุด/ต่อใหม่ และคำสั่งที่เหมาะกับแต่ละ role บน staging.
- มีผู้ดูแลติดตาม logs และวิธี rollback release. โดเมนและ TLS สำหรับการเข้าจากเครือข่ายยังต้องกำหนดก่อนเริ่ม pilot ระยะไกล.
