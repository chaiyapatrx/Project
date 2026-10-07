# ติดตั้ง AUCC ใน LAN ด้วย HTTPS/WSS

คู่มือนี้สำหรับตั้งค่า LAN ด้วยตนเอง หากใช้ Setup รุ่น 1.0.0 ให้เริ่มจาก [README.md](README.md) ส่วนการสร้าง DB, migration และ build อยู่ใน [MANUAL_SETUP.md](MANUAL_SETUP.md) ตัวอย่างด้านล่างใช้ `server.aucc.lan` / `192.168.1.10` เปลี่ยนเป็นชื่อและ IP ของระบบจริง ไม่มีการผูกโปรเจกต์กับ IP เครื่องพัฒนาเดิม

## 1. เตรียม server และ DNS

กำหนด IP คงที่หรือ DHCP reservation ให้ server ตั้ง DNS ภายในให้ client แปลง `server.aucc.lan` เป็น IP นี้ได้ ถ้าใช้ IP ตรง certificate ต้องมี IP ใน Subject Alternative Name (SAN) ทดสอบการเข้าถึงจากเครื่อง client

server ใช้ MySQL/MariaDB กับ Go backend ไม่ต้องเปิด Apache ของ XAMPP เว็บที่ build แล้วให้บริการโดย Go ผ่าน `FRONTEND_DIST_DIR` อย่าเปิดพอร์ต DB ให้ client ทั่วไป

## 2. ใบรับรอง

เลือกใบรับรองจาก CA ที่องค์กร/Windows เชื่อถือ หรือขอผู้ดูแลออก certificate สำหรับชื่อ server ตัวอย่างสร้าง CA ภายในด้วย OpenSSL ต่อไปนี้ใช้สำหรับห้องทดลอง ผู้รันต้องเก็บ CA private key และ passphrase อย่างปลอดภัย

สร้างโฟลเดอร์ certificate **นอก Git** เช่น `%LOCALAPPDATA%\AUCC Server\tls` แล้วเปิด PowerShell ในโฟลเดอร์นั้น สร้าง `server.ext`:

```ini
basicConstraints=critical,CA:FALSE
keyUsage=critical,digitalSignature,keyEncipherment
extendedKeyUsage=serverAuth
subjectAltName=DNS:server.aucc.lan,DNS:localhost,IP:192.168.1.10,IP:127.0.0.1
```

สร้างและลงนาม (ต้องมี OpenSSL; `ca.key` จะถาม passphrase):

```powershell
openssl genrsa -aes256 -out ca.key 3072
openssl req -x509 -new -sha256 -key ca.key -out ca.pem -days 3650 -subj '/CN=AUCC Lab CA' -addext 'basicConstraints=critical,CA:TRUE' -addext 'keyUsage=critical,keyCertSign,cRLSign'
openssl genrsa -out server.key 3072
openssl req -new -key server.key -out server.csr -subj '/CN=server.aucc.lan'
openssl x509 -req -in server.csr -CA ca.pem -CAkey ca.key -CAcreateserial -out server.pem -days 365 -sha256 -extfile server.ext
openssl verify -CAfile ca.pem server.pem
```

ตรวจ SAN ให้ตรง URL ทุกเครื่อง ส่งต่อเฉพาะ `ca.pem` ที่เป็น public certificate ผ่านช่องทางที่เชื่อถือ ตรวจ fingerprint กับผู้ดูแลก่อนเชื่อถือ ห้ามส่ง `ca.key` หรือ `server.key` ให้ client หรือคัดลอกลง installer ใบรับรองต้องต่ออายุก่อนหมดและเวลาของเครื่องต้องถูกต้อง

สำหรับ browser ให้ผู้ดูแลติดตั้ง public CA ใน Trusted Root Certification Authorities ของผู้ใช้/เครื่องที่ใช้งาน ตรวจ fingerprint ก่อน ตัวอย่างต่อผู้ใช้:

```powershell
certutil -user -addstore Root C:\trusted-path\ca.pem
```

Agent สามารถใช้ `server_ca_file` ที่ installer ใส่ให้โดยไม่ต้องเปลี่ยน trust store ของ Windows

## 3. ตั้ง `.env` และ build เว็บ

เปลี่ยนค่าตัวอย่างโดยคง DB/JWT/admin ของตนไว้:

```dotenv
HTTP_ADDR=0.0.0.0:8000
TLS_CERT_FILE=C:/Users/ชื่อบัญชี/AppData/Local/AUCC Server/tls/server.pem
TLS_KEY_FILE=C:/Users/ชื่อบัญชี/AppData/Local/AUCC Server/tls/server.key
COOKIE_SECURE=true
COOKIE_SAMESITE=lax
COOKIE_DOMAIN=
ALLOWED_ORIGINS=https://server.aucc.lan:8000,https://192.168.1.10:8000,https://localhost:8000
TRUSTED_PROXIES=
FRONTEND_DIST_DIR=../frontend/dist
AGENT_RELEASE_DIR=agent-releases
```

Backend ปฏิเสธ plain HTTP ที่ bind ออกเครือข่าย หากใช้ reverse proxy ให้ backend ฟัง loopback ส่วน proxy จัดการ HTTPS/WSS กำหนด TRUSTED_PROXIES เฉพาะ proxy จริง และใช้ secure cookie

Build frontend แบบ origin เดียวตาม README อย่าติดค่า `http://localhost:8000` จากการทดลองลง production bundle เริ่ม backend แล้วเปิด `https://server.aucc.lan:8000/health/ready` ต้องตอบ 200 ด้วย certificate ที่เชื่อถือ ไม่ใช้ `-k` เพื่อข้ามการตรวจ TLS

## 4. Firewall

เปิด PowerShell แบบผู้ดูแลบน server กำหนด subnet ของ client จริง ตัวอย่าง:

```powershell
powershell -NoProfile -File enable-aucc-lan-firewall.ps1 -RemoteAddress '192.168.1.0/24' -Port 8000
```

สคริปต์สร้าง/อัปเดตกฎชื่อ `AUCC HTTPS 8000 LAN` เปิดเฉพาะ network profile **Private** ตรวจว่า Windows network อยู่ profile ที่องค์กรอนุญาต เปลี่ยน subnet แล้วรันใหม่เพื่อปรับกฎ ไม่เปิด Public ทั้งหมดอัตโนมัติ

## 5. สร้าง installer สำหรับเครือข่ายนี้

บนเครื่อง build จาก root:

```powershell
New-Item -ItemType Directory client-go/dist -Force
Copy-Item 'C:\trusted-path\ca.pem' client-go/dist/server-ca.pem
$env:AUCC_SERVER_URL = 'wss://server.aucc.lan:8000/api/ws/agent'
cmd /c client-go\build_installer.bat
Remove-Item Env:AUCC_SERVER_URL
```

ใช้ CA ของ server จริงเท่านั้น ถ้าใช้ public CA และไม่ต้องการ bundle CA ภายใน ให้นำ `dist/server-ca.pem` เก่าออกก่อน build ส่ง `AUCCAgentSetup-<VERSION>.exe` ไป client ไม่ต้องส่ง `.env` หรือ config/admin/DB password

## 6. ติดตั้งและรับรองผล

1. Sign in บัญชี Windows ที่จะใช้ Agent ติดตั้ง Setup เลือกพาธที่บัญชีเขียนได้
2. Agent ส่ง enrollment เข้า server เปิด Admin → Computer Management → Pending agents ตรวจชื่อเครื่อง/HWID แล้ว Approve
3. เปิด Agent ต้องเห็นหน้าล็อกและสถานะ online บนหน้า Admin
4. ทดสอบบัญชี AUCC เมื่อไม่มี booking และทดสอบ Access Code เมื่อมี booking บัญชีทั่วไปต้องไม่เปิดสถานีที่ถูกจอง
5. ทดสอบหมดเวลา/logout/หลุดเครือข่าย/restart backend/ปิดบัญชี/อัปเดตและ rollback บนเครื่องทดลองก่อนกระจายทั้งหมด
6. ตรวจ backup/restore บน staging และสำรอง `agent-releases` กับ DB คู่กัน

Agent autostart ต่อบัญชีจาก Windows Run registry การย้าย server URL หรือ CA ต้องแก้ config เดิมด้วย การลง Setup ใหม่รักษา config จึงไม่แทนค่าทั้งหมดอัตโนมัติ

## 7. การเปิดหลัง reboot

ถ้าใช้ launcher ให้สร้าง shortcut ชี้ `start-aucc.cmd` ใน Startup ของ Windows account บน server สคริปต์ต้องยังอยู่กับโปรเจกต์และ build ปัจจุบัน การเริ่มแบบนี้ทำงานหลัง sign in เท่านั้น

ถ้าระบบต้องทำงานก่อน sign in ให้ผู้ดูแลติดตั้ง **backend/DB** เป็น services พร้อม working directory/config/logs ที่ถูกต้อง ส่วน **Agent overlay ต้องอยู่ interactive user session** อย่าย้าย Agent ไป service แล้วคาดหวังว่า overlay จะปรากฏบน desktop

ระบบนี้ยังไม่มีผล load benchmark และผลติดตั้งทุก Windows policy ดูหลักฐานที่ทำจริงและความเสี่ยงค้างใน [AUDIT.md](AUDIT.md)
