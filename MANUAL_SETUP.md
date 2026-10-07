# AUCC — คู่มือพัฒนาและติดตั้งด้วยตนเอง

คู่มือนี้สำหรับผู้พัฒนาหรือผู้ดูแลที่จัดการฐานข้อมูลและเครือข่ายเอง การติดตั้งผ่าน Setup รุ่น 1.0.0 ให้เริ่มจาก [README.md](README.md) หรือ [คู่มือตัวติดตั้ง](installer/README.md) ขั้นตอนอนุมัติ Agent ด้วยมือด้านล่างใช้เมื่อไม่ได้ใช้ชุด Client ส่วนตัวที่เครื่องแม่สร้างให้

คู่มือปัจจุบันปรับปรุงวันที่ 6 ตุลาคม 2026 ใช้ Go backend, React frontend, MySQL/MariaDB และ Agent สำหรับ Windows รุ่น Agent อ่านจาก `client-go/VERSION` ผลตรวจและข้อจำกัดอยู่ใน [AUDIT.md](AUDIT.md) การติดตั้งหลายเครื่องอยู่ใน [LAN_SETUP.md](LAN_SETUP.md)

สำหรับติดตั้ง Windows แบบอัตโนมัติ ใช้ [ตัวติดตั้ง Server และ Client](installer/README.md) แทนขั้นตอน setup ด้วยตนเองด้านล่าง

## 1. โครงสร้างและการทำงาน

```text
Browser (React) ── HTTPS REST + WSS monitor ── Go backend ── MySQL/MariaDB
                                                  │
                                           WSS + signed commands
                                                  │
                                         Windows AUCC Agent
```

| ส่วน | ไฟล์หลัก | หน้าที่ |
|---|---|---|
| Backend | `backend-go/cmd/server/main.go` | โหลดค่าตั้งต้น ต่อฐานข้อมูล เปิด API และ WebSocket |
| สิทธิ์และบัญชี | `backend-go/internal/auth`, `internal/middleware`, `internal/handler/auth.go` | JWT, cookie, CSRF, role, password hash และการเพิกถอน token |
| การจอง/สถานี | `backend-go/internal/handler/booking.go`, `station_auth.go`, `computer.go` | ตรวจสิทธิ์ จอง เปิด session และสั่งเครื่อง |
| สถานะสด | `backend-go/internal/hub`, `internal/worker` | เก็บสถานะการเชื่อมต่อใน RAM และปิด session หมดเวลา |
| ฐานข้อมูล | `migrations` | SQL ตามลำดับรุ่น และ ledger `schema_migrations` |
| หน้าเว็บ | `frontend/src` | หน้าตาม role, API helper และการรับสถานะจาก WebSocket |
| Agent | `client-go/main.go`, `station_webview.go`, `station_login.html` | แสดงหน้าล็อก รับคำสั่ง และยืนยันตัวตนกับ backend |
| อัปเดต Agent | `client-go/update.go`, `client-go/updater` | ดาวน์โหลด ตรวจไฟล์ เปลี่ยน EXE และคืนรุ่นก่อนเมื่อเปิดไม่ผ่าน |
| ดูแล Windows server | `start-aucc`, `stop-aucc`, `backup-db`, `scripts` | เปิด/หยุด backend และสำรองฐานข้อมูล |

### สิทธิ์ผู้ใช้

- **Admin:** จัดการผู้ใช้/สถานี อนุมัติ Agent ตั้งค่าระบบ อัปโหลดรุ่น Agent ดูข้อมูลและสั่งเครื่อง
- **Staff:** ดูสถานี/การจอง/ประวัติ สั่งเครื่อง และปล่อยหรือพักรุ่น Agent ที่ Admin อัปโหลดไว้
- **Executive:** ดูสถิติและรายงาน ไม่มีสิทธิ์สั่งเครื่องหรือแก้ค่าระบบ
- **Student:** ดูสถานี จองเครื่อง ดู/ยกเลิก/ต่อเวลาการจองของตนตามเงื่อนไข
- บุคคลทั่วไปดูรายการสถานีได้ แต่ข้อมูลผู้ใช้และการควบคุมเครื่องต้องผ่านการยืนยันตัวตน

### ลำดับการใช้งานจริง

1. Agent สร้าง credential ประจำสถานีและ HWID ส่งคำขอลงทะเบียน สถานีรอ Admin อนุมัติก่อนใช้งาน
2. เมื่อเชื่อมต่อ Agent เริ่มในสถานะล็อก Backend ตรวจ credential, HWID, การอนุมัติและสถานะสถานี
3. เครื่องว่างที่ไม่มี booking ให้กรอกบัญชี AUCC เพื่อเปิด walk-in session
4. การจองในระบบนี้ **เริ่มทันที** ไม่ใช่การจองช่วงวันเวลาล่วงหน้า Backend ล็อกแถวสถานีและผู้ใช้ใน transaction ป้องกันการจองซ้อน และสร้าง Access Code 6 หลัก
5. เครื่องที่จองแล้วแสดงฟอร์ม Access Code ผู้ใช้กรอก code ของ booking บนเครื่องนั้น จึงปลดล็อกได้
6. Backend บันทึก `usage_logs` และเวลาสิ้นสุด session ส่วน Hub เก็บสถานะสดใน RAM Agent ตรวจ HMAC ของคำสั่ง
7. เมื่อหมดเวลา/ยกเลิก/สั่ง LOGOUT ระบบปิด session และคืนหน้าล็อก เมื่อการเชื่อมต่อหลุด Agent ล็อกหน้าจอ
8. Backend เริ่มใหม่แล้วโหลด booking/session ที่ยังเปิดจากฐานข้อมูล สถานะ online ต้องรอ Agent เชื่อมต่อใหม่

`LOCK` บังหน้าจอโดยอาจคง session ไว้ ส่วน `LOGOUT` จบ session จึงต้องเลือกคำสั่งให้ตรงงาน เวลาตั้งต้นสูงสุดต่อรอบ 120 นาที เปลี่ยนได้ใน Settings ช่วง 30–240 นาที

**ขอบเขตหน้าล็อก:** Agent ทำงานใน Windows session ของผู้ใช้ที่ sign in แล้ว เป็น overlay ระดับแอป ผู้ที่ปิด process หรือเปลี่ยน Windows session อาจหลบหน้าล็อกได้ เครื่องสาธารณะต้องใช้บัญชี Windows สิทธิ์จำกัดและนโยบายควบคุมเครื่องที่เหมาะสม

## 2. สิ่งที่ต้องติดตั้ง

เครื่องพัฒนา/สร้าง release: Go ตาม `go.mod` (1.26 ขึ้นไป), Node.js 22.12 ขึ้นไปกับ npm, Python 3.10 ขึ้นไป, MySQL 8 หรือ MariaDB ที่รองรับ schema นี้ และ Inno Setup 6/7 สำหรับสร้าง Windows installer

เครื่อง server ที่ใช้ไฟล์ build แล้ว: ฐานข้อมูลและ backend EXE ไม่ต้องมี Node หรือ Go; Python/PyMySQL ใช้ตอน migration เครื่อง client: Windows 10/11 แบบ x64, แนะนำให้มี Microsoft Edge WebView2 Runtime เพื่อใช้หน้าล็อกแบบเว็บ Agent มี native fallback

ตรวจเครื่องมือใน PowerShell:

```powershell
go version
node --version
npm --version
python --version
```

## 3. เตรียมฐานข้อมูลและค่าตั้งต้น

ทำจากโฟลเดอร์โปรเจกต์ อย่าคัดลอกทับ `.env` ที่ใช้งานอยู่:

```powershell
Copy-Item .env.example .env
python -c "import secrets; print(secrets.token_urlsafe(48))"
```

นำค่าที่สุ่มได้ใส่ `JWT_SECRET` แล้วกรอก `.env` ตัวอย่างสำหรับเครื่องเดียว:

```dotenv
DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=aucc_app
DB_PASS=
DB_NAME=aucc
JWT_SECRET=
BOOTSTRAP_ADMIN=false
SADMIN_USERNAME=
SADMIN_PASSWORD=
HTTP_ADDR=127.0.0.1:8000
COOKIE_SECURE=false
COOKIE_SAMESITE=lax
ALLOWED_ORIGINS=http://localhost:5173,http://127.0.0.1:5173
SELF_REGISTRATION_ENABLED=false
ALLOW_LEGACY_AGENTS=false
```

ค่าลับที่เว้นว่างต้องกรอกจริงก่อนเปิด backend ไม่มีรหัสผ่านเริ่มต้นที่แจกมากับระบบ การตั้ง Admin ครั้งแรกให้เปิด `BOOTSTRAP_ADMIN=true` ชั่วคราว ตั้งชื่อบัญชีจริงและรหัสผ่านสุ่มเฉพาะระบบ อย่างน้อย 15 ตัวอักษรและไม่เกิน 72 UTF-8 bytes ตัวอย่างคำสั่งสร้างรหัสบนเครื่องของผู้ติดตั้ง:

```powershell
python -c "import secrets; print(secrets.token_urlsafe(24))"
```

เมื่อบัญชีถูกสร้างแล้ว ให้ตั้ง `BOOTSTRAP_ADMIN=false` และลบ `SADMIN_USERNAME`/`SADMIN_PASSWORD` จาก `.env`/secret store การลบค่าตั้งต้นไม่ลบบัญชีที่สร้างแล้ว ฐานข้อมูลเก็บเฉพาะ bcrypt hash ระบบไม่สร้างบัญชีเพิ่มจากการเปลี่ยนชื่อตั้งต้น และเก็บ marker `admin_bootstrap_completed` เพื่อไม่สร้างบัญชีกลับหลังลบ/ปิดบัญชี หากตั้งค่าตั้งต้นหรือ DB ล้มเหลว bootstrap หยุดด้วย error ไม่เปิดแบบเงียบ ๆ

อย่าใช้ข้อความในคู่มือเป็นรหัสจริง และอย่าเก็บ Admin password ไว้ใน environment ของ runtime ระยะยาว ตั้งสิทธิ์อ่าน `.env` ให้เฉพาะบัญชี service/ผู้ดูแล; `.gitignore` ป้องกัน Git ไม่ได้ป้องกันผู้ใช้ Windows คนอื่นอ่านไฟล์

บน Windows จำกัดสิทธิ์ `.env`, config สำเนาและ backup ก่อนใช้งาน:

```powershell
powershell -NoProfile -File scripts/protect-secrets.ps1
# จำกัด deployment folder ด้วย เพื่อไม่ให้ผู้ใช้อื่นแทนที่ config/EXE ผ่าน parent directory
powershell -NoProfile -File scripts/protect-secrets.ps1 -Path 'D:\private\AUCC'
# ถ้า backend รันด้วยบัญชี service อื่น ระบุบัญชีจริงแทนผู้ใช้ปัจจุบัน
powershell -NoProfile -File scripts/protect-secrets.ps1 -ServiceAccount 'DOMAIN\aucc-service'
# private key ที่อยู่นอกโปรเจกต์ ให้ระบุพาธจริงเพิ่มเติม
powershell -NoProfile -File scripts/protect-secrets.ps1 -Path 'C:\private\tls\server.key'
```

อ่านไฟล์ได้เฉพาะบัญชีที่เลือก, SYSTEM และ Administrators โฟลเดอร์ที่เก็บไฟล์ต้องไม่เปิดให้บุคคลอื่นเขียน/แทนที่ไฟล์ Secret manager ขององค์กรใช้ส่งค่าผ่าน environment ได้ แต่ต้องไม่ส่งลง frontend, build log หรือ command line

รหัสใหม่ของบัญชีในระบบต้องอย่างน้อย **15 ตัวอักษร** และไม่เกิน **72 UTF-8 bytes** ตามข้อจำกัด bcrypt ระบบไม่ปิดบัญชีเก่าทันทีเมื่อเพิ่ม policy นี้ ผู้ดูแลต้องทบทวนบัญชีเก่าและเปลี่ยนรหัสที่เดาง่าย ไม่มี MFA ในระบบปัจจุบัน จึงต้องจำกัดเครือข่ายเข้า Admin และผ่าน security review ก่อนเปิดสาธารณะ

ระบบจำกัดการตรวจรหัสบัญชีเดียวกัน 10 ครั้งต่อนาทีร่วมทั้งเว็บ/สถานี/ยืนยันรหัสเดิม การเปลี่ยน IP ไม่รีเซ็ตจำนวนครั้ง การจำกัดนี้อยู่ใน process เดียวและอาจทำให้บัญชีถูกพักชั่วคราวหนึ่งนาทีเมื่อถูกโจมตี ถ้ามีหลาย backend instances ต้องใช้ shared limiter ที่ deployment layer

สร้าง database ก่อน migration โดยใช้บัญชีผู้ดูแลฐานข้อมูล ตัวอย่าง SQL สำหรับ server/DB เครื่องเดียว เปลี่ยนรหัสตัวอย่างก่อนรัน:

```sql
CREATE DATABASE aucc CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'aucc_app'@'127.0.0.1' IDENTIFIED BY 'REPLACE_WITH_STRONG_PASSWORD';
GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, INDEX, REFERENCES
ON aucc.* TO 'aucc_app'@'127.0.0.1';
```

host ของผู้ใช้ DB ต้องตรงกับการเชื่อมต่อที่ฐานข้อมูลตรวจจริง งาน migration ควรใช้บัญชีที่มี DDL rights ส่วนบัญชี runtime หลัง migration ใช้เฉพาะ SELECT/INSERT/UPDATE/DELETE ได้ งาน backup ที่รวม routines/triggers อาจต้องใช้บัญชีสำรองข้อมูลที่มีสิทธิ์เพิ่มตามรุ่นฐานข้อมูล

### ค่าที่ต้องเข้าใจ

| ค่า | ความหมาย |
|---|---|
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASS`, `DB_NAME` | Backend, migration และ backup ต้องชี้ฐานเดียวกัน DB_PASS ต้องไม่ว่าง |
| `DB_TLS_CA_FILE` | CA ที่ผู้ดูแล DB ให้มาเมื่อฐานอยู่ต่างเครื่อง ต้องตรวจชื่อ host และ certificate |
| `JWT_SECRET` | อย่างน้อย 32 bytes เปลี่ยนแล้ว token เดิมใช้ไม่ได้ |
| `JWT_EXPIRATION_MINUTES` | อายุ token จำนวนเต็มบวก ค่าเริ่มต้น 1440 นาที |
| `BOOTSTRAP_ADMIN` | false โดยค่าเริ่มต้น เปิด true เฉพาะ setup ครั้งแรก ต้องใช้คู่กับ SADMIN credentials |
| `SADMIN_USERNAME`, `SADMIN_PASSWORD` | ใช้เฉพาะ bootstrap ครั้งแรก ไม่มี default password ลบจาก config หลังสร้าง ไม่ใช้ reset/recreate บัญชี |
| `HTTP_ADDR` | address ที่ backend ฟัง เช่น `127.0.0.1:8000` หรือ `0.0.0.0:8000` เมื่อมี TLS |
| `TLS_CERT_FILE`, `TLS_KEY_FILE` | ต้องตั้งคู่กันสำหรับ HTTPS โดยตรง ใช้ absolute paths ได้ |
| `COOKIE_SECURE` | true สำหรับ HTTPS เครือข่าย; false เฉพาะ local HTTP |
| `COOKIE_SAMESITE`, `COOKIE_DOMAIN` | ใช้ lax และปล่อย domain ว่างเมื่อเว็บ/API อยู่ origin เดียวกัน |
| `ALLOWED_ORIGINS` | origin ของเว็บ คั่นด้วย comma รวม scheme/port ให้ถูก ไม่ใช่ URL พร้อม path |
| `TRUSTED_PROXIES` | IP/CIDR ของ reverse proxy ที่เชื่อถือ ปล่อยว่างถ้าไม่มี proxy |
| `FRONTEND_DIST_DIR` | ถ้าตั้ง backend ให้บริการเว็บ build ด้วย เช่น `../frontend/dist` เมื่อรันจาก backend-go |
| `AGENT_RELEASE_DIR` | storage EXE ที่อัปโหลด ต้องเป็นที่ถาวรและสำรองพร้อม DB |
| `SELF_REGISTRATION_ENABLED` | false แล้ว Admin สร้างบัญชีให้ ถ้า true เปิดสมัครเฉพาะสิทธิ์นักศึกษา |
| `ALLOW_LEGACY_AGENTS`, `AGENT_SECRET` | ใช้เฉพาะย้าย Agent รุ่นเก่าที่ใช้ secret กลาง ปกติปิดไว้ |
| `VITE_API_BASE_URL` | frontend build/dev อ่านค่านี้ตอน build การแก้ไฟล์หลัง build ต้อง build ใหม่ |

พาธสัมพันธ์ backend อ้างจาก working directory สคริปต์ start รัน backend ภายใน `backend-go` อย่าใส่ DB/admin secret ในค่าที่ขึ้นต้นด้วย `VITE_` เพราะจะถูกส่งให้ browser

### Migration

```powershell
python -m pip install -r migrations/requirements.txt
python migrations/apply_migrations.py
```

runner อ่าน `.env` ที่ root (environment variables มีลำดับก่อน), ใช้ verified TLS เมื่อ DB อยู่ต่างเครื่อง, ล็อกการ migration บน connection และรัน `000001` ถึง `000007` ตาม ledger รองรับการเริ่มใหม่หลังบาง DDL ทำแล้วแต่ ledger ยังไม่บันทึก ไม่ควรแก้เนื้อหา SQL รุ่นที่ใช้แล้ว; การเปลี่ยน schema ครั้งต่อไปให้เพิ่ม migration รุ่นใหม่

**สำรองก่อน upgrade:** MySQL DDL อาจ commit ทันที การ rollback transaction ไม่คืน schema ทั้งหมด ใช้ฐานสำเนาทดสอบก่อน `.down.sql` บางไฟล์ลบข้อมูลและไม่ได้ใช้โดย runner อัตโนมัติ เก็บไว้สำหรับผู้ดูแลที่ตรวจผลแล้วเท่านั้น

## 4. ทดลองในเครื่องเดียว

เปิด PowerShell แยกสองหน้าต่าง:

```powershell
# หน้าต่างที่ 1 จากโฟลเดอร์โปรเจกต์
Set-Location backend-go
go run ./cmd/server
```

```powershell
# หน้าต่างที่ 2 จากโฟลเดอร์โปรเจกต์
Set-Location frontend
Copy-Item .env.example .env.local
npm ci
npm run dev
```

ตรวจ `http://localhost:8000/health/ready` ต้องตอบ 200 แล้วเปิด `http://localhost:5173` เข้าด้วยบัญชี bootstrap Admin ถ้า backend/agent อยู่เครื่องเดียวกันจึงใช้ `ws://localhost:8000/api/ws/agent` ได้ การกด Ctrl+C ในหน้าต่าง backend เรียกขั้นตอน shutdown ของ Go

## 5. Build สำหรับ server

จาก root ใน PowerShell:

```powershell
Set-Location frontend
npm ci
# เว็บ/API origin เดียวกัน ป้องกัน .env.local จากการทดลองติดไปใน build
$env:VITE_API_BASE_URL = ''
npm run lint
npm run build
Remove-Item Env:VITE_API_BASE_URL
Set-Location ../backend-go
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
go build -trimpath -o AUCCServer.exe ./cmd/server
Set-Location ..
```

ตั้ง `FRONTEND_DIST_DIR=../frontend/dist` ใน `.env` แล้วใช้ `start-aucc.cmd` ตัว backend เปิด API และเว็บบน port เดียวกัน ขั้นตอน HTTPS/LAN อยู่ใน [LAN_SETUP.md](LAN_SETUP.md)

`start-aucc.ps1` อ่าน DB host/port จริง ถ้าเป็นฐานในเครื่องและ port ยังไม่เปิด จะเปิด XAMPP MySQL จาก `C:\xampp\mysql\bin` ค่า `-MySQLBin` เปลี่ยนได้ ถ้าใช้ DB service อื่นให้เปิด service ก่อน สคริปต์ตรวจ port เท่านั้น ความพร้อมจริงต้องตรวจ `/health/ready` และ `backend-go/server.err.log`

`stop-aucc.cmd` หยุดเฉพาะ EXE ของโปรเจกต์นี้ บน Windows เป็นการ terminate process ทันที ถ้าต้องการ shutdown ตามขั้นตอนให้รัน backend foreground แล้วกด Ctrl+C ฐานข้อมูลยังทำงานต่อ ถ้าต้องหยุดฐาน **ในเครื่องที่ใช้งานร่วมกันให้ตรวจผลต่อโปรแกรมอื่นก่อน**:

```powershell
powershell -NoProfile -File stop-aucc.ps1 -StopMySQL
```

ใช้ `mysqladmin shutdown` ไม่บังคับ kill MySQL หากบัญชีแอปไม่มี SHUTDOWN privilege ให้หยุดจาก XAMPP/service manager

## 6. สร้างและติดตั้ง Agent

สร้าง installer จากเครื่อง build ใน PowerShell:

```powershell
$env:AUCC_SERVER_URL = 'wss://server.aucc.lan:8000/api/ws/agent'
# สำหรับ CA ภายใน ให้คัดลอกเฉพาะ public CA ลง client-go/dist/server-ca.pem ก่อน build
cmd /c client-go\build_installer.bat
Remove-Item Env:AUCC_SERVER_URL
```

ผลลัพธ์ใน `client-go/dist`: `AUCCAgent.exe`, `AUCCUpdater.exe` และ `AUCCAgentSetup-<VERSION>.exe` เลขรุ่นล่าสุดดูจาก `client-go/VERSION` หากไม่ตั้ง AUCC_SERVER_URL ตัวติดตั้งจะถาม URL การ build ด้วย public CA ไม่ใส่ private key ลง installer

บน client ให้ sign in ด้วย Windows account ที่จะใช้ Agent แล้วเปิด installer ติดตั้งต่อผู้ใช้ที่ `%LOCALAPPDATA%\Programs\AUCC Agent` config อยู่ที่ `%LOCALAPPDATA%\AUCC Agent\config.json` มี autostart ของบัญชีนี้เมื่อ sign in ตัวติดตั้งส่ง enrollment แต่การเปิด Agent ทันทีต้องเลือกในหน้าสุดท้าย จากหน้า Admin ตรวจชื่อเครื่อง/HWID แล้ว Approve ก่อนทดสอบ

การติดตั้งใหม่บนบัญชีเดิมรักษา config ไว้ URL เดิมจึงไม่เปลี่ยนตาม build ใหม่อัตโนมัติ ถ้าย้าย server ต้องแก้ `server_url`/`server_ca_file` ใน config อย่างถูกต้องและเริ่ม Agent ใหม่ การ uninstall ลบ config ของบัญชีนั้นด้วย

พัฒนาแบบไม่ใช้ installer: `client-go/build_agent.bat` สร้าง EXE คู่กัน คัดลอก `config.example.json` เป็น `config.json` ใกล้ EXE แล้วแก้ URL แต่ config ใน LOCALAPPDATA ถ้ามีอยู่จะถูกเลือกก่อน

## 7. อัปเดตและย้อนรุ่น Agent

1. เพิ่มเลขรุ่นใน `client-go/VERSION` แล้ว build ใหม่ ทดสอบ installer/Agent บนเครื่องทดสอบ
2. Admin เปิด Agent updates อัปโหลด **AUCCAgent.exe** พร้อมเลขรุ่นตรงกับ VERSION ห้ามอัปโหลด Setup แทน
3. Admin/Staff กด Roll out เครื่องเช็ก release ประมาณทุกหนึ่งนาที เครื่องมี session รอจนว่าง
4. Agent ตรวจขนาด SHA-256 และ updater ตรวจเลขรุ่น EXE ก่อนเปลี่ยน หากรุ่นใหม่เปิดไม่ผ่านช่วงตรวจเริ่มต้น updater คืน EXE ก่อนหน้า
5. Pause หยุดการปล่อยต่อ ไม่ย้อนเครื่องที่เปลี่ยนไปแล้ว ถ้าต้องย้อนให้ Roll out รุ่นก่อนที่เก็บไว้

สำรอง release storage และ DB คู่กัน รุ่นที่ยังอาจ rollback ต้องเก็บไว้ Updater เองไม่ได้เปลี่ยนผ่านการอัปเดต Agent ต้องติดตั้ง Setup ใหม่เมื่อ updater เปลี่ยน ตรวจ `update.log` และไฟล์แจ้งอัปเดตล้มเหลวในโฟลเดอร์ติดตั้ง

Agent รุ่นปัจจุบันอัปเดตอัตโนมัติเฉพาะ HTTPS/WSS ที่ตรวจ certificate แม้ server อยู่ localhost การทดลอง `ws://localhost` ยังใช้ login/คำสั่งเพื่อพัฒนาได้ แต่ไม่ใช้ auto-update ติดตั้งไฟล์ใหม่ผ่าน Setup ที่ไว้ใจได้แทน

## 8. Backup และ restore

```powershell
powershell -NoProfile -File backup-db.ps1 -RetentionDays 14
```

อ่าน `.env`/environment จริง ไม่ส่งรหัสผ่านใน command line ใช้ native mysqldump เขียนไฟล์โดยตรงเพื่อไม่ให้ PowerShell เปลี่ยน encoding ไฟล์ที่ยังไม่สำเร็จเป็น `.partial` และไม่ใช้ลบ backup เก่า เมื่อสำเร็จจึงเปลี่ยนเป็น `.sql` แล้วลบเฉพาะ backup ของ DB นี้ที่เกินอายุใน `backups` ย้ายสำเนาไป storage แยกและจำกัดสิทธิ์เข้าถึง

dump ใช้ `--databases` จึงมีคำสั่งสร้าง/เลือกชื่อ DB เดิม **อย่า import ใส่เครื่อง production เพื่อทดลอง** บน staging แยก ใช้ MySQL CLI แบบ interactive เพื่อเลี่ยง PowerShell input redirection/encoding:

```text
mysql --host=127.0.0.1 --user=ผู้ดูแลstaging -p
mysql> SOURCE D:/path/to/aucc_backup_วันที่.sql;
```

ตรวจจำนวนผู้ใช้/สถานี/booking/usage และ ledger หลัง restore แล้วรัน backend ด้วย `.env` ของ staging สำเนา SQL มีข้อมูลผู้ใช้และ password hash จึงต้องป้องกันเหมือนข้อมูลจริง

## 9. ตรวจสอบก่อนใช้งาน

```powershell
Set-Location backend-go
go test ./...
go vet ./...
Set-Location ../client-go
go test ./...
go vet -unsafeptr=false ./...
Set-Location ../frontend
npm ci
npm run lint
npm test
npm run build
npm audit --omit=dev
Set-Location ..
python -m unittest discover -s migrations -v
powershell -NoProfile -File scripts/test-operations.ps1
git diff --check
```

Client full `go vet` แจ้ง cast Win32 LPARAM เป็น pointer ใน callback ดูข้อจำกัดใน AUDIT ไม่ควรอ้างว่า full vet ผ่าน ตรวจ dependency Go ด้วย `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` ในแต่ละ module ผล scan อาจเปลี่ยนตาม advisory วันที่ตรวจ

ทดสอบบน VM/เครื่องจริง: การอนุมัติ, walk-in, code ผิด/ถูก, จองซ้อน, logout, ต่อเวลา, หมดเวลา, WebSocket หลุด/กลับ, backend restart, บัญชีถูกปิด, อัปเดต/rollback และ backup/restore ผล unit/build ไม่แทนการทดสอบนี้

## 10. แก้ปัญหาเบื้องต้น

| อาการ | ตรวจอะไร |
|---|---|
| Backend เปิดไม่ได้ | `.env`, DB user/port, migration ledger, TLS paths, `server.err.log` |
| readiness ไม่ผ่าน | DB ต้องเข้าถึงได้จริง ไม่ใช่แค่เปิด TCP port |
| เว็บหน้าโล่ง/API ผิดเครื่อง | `FRONTEND_DIST_DIR`, `frontend/dist`, VITE_API_BASE_URL ที่ฝังตอน build แล้ว build ใหม่ |
| login ใช้ไม่ได้จากอีกเครื่อง | HTTPS, trusted CA, secure cookie, origin allowlist และ URL ต้องตรงกัน |
| Agent offline | URL ลงท้าย `/api/ws/agent`, CA/วันเวลาเครื่อง, firewall, Pending approval, config ที่ใช้จริง |
| ชื่อเครื่องชน | ตั้งชื่อ Windows/station ให้ไม่ซ้ำ ตรวจ HWID ก่อนอนุมัติ |
| booking ไม่ได้ | สถานะ available + online, ไม่มี session เดิม, maintenance mode, ข้อจำกัดการจองผู้ใช้ |
| Agent ไม่อัปเดต | active release, รุ่นต่างจากเครื่อง, ไม่มี session, directory เขียนได้, updater อยู่ใกล้ EXE |
| Agent ไม่เปิดหลัง boot | autostart เริ่มหลัง Windows account นั้น sign in ไม่ใช่ service ก่อนล็อกอิน |

## 11. ไฟล์ปัจจุบันและการเก็บข้อมูล

คู่มือหลักใช้ README และ LAN_SETUP รายงานตรวจล่าสุดใช้ AUDIT ลบแผนย้อนหลัง/รายงานเปอร์เซ็นต์เก่า, สคริปต์ทดสอบ admin ที่ hardcode IP/รุ่น และ combined SQL ที่ซ้ำกับ migration แล้ว ยังคง numbered migrations, regression tests, `.env.example`, `config.example.json` และ package lockfiles เพราะใช้ติดตั้งและตรวจระบบ

`.env`, config สถานี, TLS private keys, database backup และ release ที่ต้องใช้ rollback เป็นข้อมูลเฉพาะเครื่อง ไม่ควรนำขึ้น Git หรือแจกให้ client การลบข้อมูลเหล่านี้ต้องพิจารณาการกู้คืนแยกจากการลบ source เก่า
