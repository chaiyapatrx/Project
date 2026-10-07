# ติดตั้ง AUCC แบบอัตโนมัติบน Windows

## ผู้ติดตั้งใช้งาน

1. นำ `dist/AUCCServerSetup-<VERSION>.exe` ไปเครื่องแม่ เปิดแล้วกดติดตั้ง อนุญาต UAC ของ Windows
2. Setup เตรียม MariaDB ส่วนตัวบน loopback พอร์ต 3308, schema, บัญชี Admin พร้อมรหัสสุ่ม, HTTPS, firewall เฉพาะ backend/LocalSubnet และ Scheduled Task ที่เริ่มหลังเปิดเครื่องก่อน sign in ไม่ต้องมี XAMPP, Go, Node, Python หรืออินเทอร์เน็ตบนเครื่องแม่
3. หน้าสุดท้ายเปิดข้อมูลล็อกอินและเว็บไซต์ เปลี่ยนรหัส Admin หลังเข้าใช้ครั้งแรก
4. คัดลอก `%ProgramData%\AUCC Server\AUCCClientSetup.exe` ไปเครื่องลูก เปิดและกดติดตั้งด้วยบัญชี Windows ที่จะใช้ Agent ไม่ต้องกรอก URL, CA หรือ credential
5. Agent เริ่มหลังติดตั้งและหลังบัญชี Windows นี้ sign in เครื่องใหม่จากชุดติดตั้งส่วนตัวได้รับอนุมัติอัตโนมัติ แต่ยังต้องล็อกอิน AUCC หรือกรอก Access Code ก่อนใช้เครื่อง

เครื่องแม่ใช้ชื่อคอมพิวเตอร์ Windows เป็นปลายทาง ไม่แก้ IP, DNS, router หรือชื่อเครื่องโดยพลการ ทุกเครื่องต้องอยู่เครือข่ายที่เข้าถึงและ resolve ชื่อเครื่องแม่ได้; เครือข่ายที่แยก client/VLAN หรือปิด name resolution ต้องให้ผู้ดูแลเครือข่ายจัดการก่อน ไม่มีตัวติดตั้งที่ข้ามข้อจำกัดเครือข่ายหรือ UAC ได้

ข้อมูลทั้งหมดอยู่ใน `%ProgramData%\AUCC Server` จำกัดสิทธิ์เฉพาะบัญชีผู้ติดตั้ง, Administrators และ SYSTEM หากติดตั้ง Server ด้วยบัญชีผู้ดูแลอีกบัญชี ข้อมูลล็อกอินอยู่ในโฟลเดอร์ของ Server ให้บัญชีนั้นเปิด

## ชุดติดตั้ง Client เป็นข้อมูลส่วนตัว

`AUCCClientSetup.exe` รวม URL, public CA และกุญแจลงทะเบียนเฉพาะ deployment ผู้ครอบครองสามารถเพิ่มสถานีใหม่ได้ แจกเฉพาะผู้ดูแล/เครื่องที่อนุญาต ไม่วางบนเว็บไซต์สาธารณะ ไม่รวมรหัส Admin, DB, JWT หรือ TLS private key เครื่องใหม่ยังมี credential แยกต่อเครื่อง; กุญแจชุดติดตั้งไม่สามารถยึดชื่อเครื่องเดิมหรือเปิดสถานีที่ถูกปิดได้

การติดตั้ง Client นี้เพิ่ม public CA ของ AUCC ลง Windows CurrentUser Root เพื่อให้เว็บเชื่อถือ certificate เฉพาะบัญชีที่ติดตั้ง ส่วน Agent ใช้ CA file ของตัวเอง ไม่มีการปิด TLS verification ใบรับรอง server อายุ 5 ปี CA อายุ 10 ปี ต้องวางแผนเปลี่ยน certificate/กระจายชุดติดตั้งใหม่ก่อนหมดอายุ

หากกุญแจชุดติดตั้งรั่ว ปิดการอนุมัติอัตโนมัติโดยลบ `AGENT_ENROLLMENT_TOKEN` จาก `.env` แล้วรัน `scripts/install-server.ps1 -Mode Stop` แบบผู้ดูแล ตามด้วย `Start-ScheduledTask -TaskName 'AUCC Server'` เครื่องที่ลงทะเบียนแล้วใช้ credential เดิมได้ การเปลี่ยนกุญแจต้องเปลี่ยน `client-package/deployment.ini` ให้ตรงแล้วรัน `scripts/install-server.ps1 -Mode Export` แบบผู้ดูแลเพื่อสร้างชุดใหม่

## Build โดยผู้พัฒนา

บนเครื่อง build ต้องมี Go ตาม go.mod, Node/npm, Python และ Inno Setup 6/7 รันจาก root:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File installer/build.ps1
```

สคริปต์ build เว็บ, backend, helper และ Agent; ดาวน์โหลด MariaDB 11.4.11 และ Python 3.13.16 จากผู้ผลิต ตรวจ SHA-256 ที่ pin ไว้ และรวม PyMySQL ตาม `migrations/requirements.txt` กับ license ของ runtime ลง Setup การติดตั้งที่ปลายทางทำงาน offline โดยใช้ migration runner เดิม ไฟล์ build/cache ถูก ignore ใน Git

แหล่ง runtime: [MariaDB ZIP](https://mariadb.com/docs/server/server-management/install-and-upgrade-mariadb/installing-mariadb/binary-packages/installing-mariadb-windows-zip-packages), [Python embeddable](https://www.python.org/downloads/release/python-31316/), [Inno Setup](https://jrsoftware.org/). รวม compiler ของ Inno Setup และ license เดิมไว้ใน payload เพื่อสร้าง Client Setup บนเครื่องแม่แบบ offline โดยใช้ template Agent เดิม

## ลงซ้ำและถอนการติดตั้ง

ลง Server ซ้ำจะหยุดเฉพาะ process/task ของ deployment นี้ รักษา DB, credentials, CA และ config เดิม แล้วใช้ migration ledger เดิม หากพอร์ต 8000/3308 มีโปรแกรมอื่นใช้อยู่จะหยุดพร้อม error โดยไม่เปลี่ยนโปรแกรมนั้น ความล้มเหลวเก็บใน `setup.log` และ log ของ backend/DB ให้รัน Setup ซ้ำหลังแก้ต้นเหตุ

การถอน Server หยุด process/task และลบ firewall rule แต่รักษา DB, credentials และ CA trust สำหรับการกู้คืน สำรองโฟลเดอร์ private ทั้งหมดก่อน upgrade; งาน migration DDL ไม่สามารถย้อนด้วย transaction ได้ทั้งหมด

Client ที่มี config เดิมจะรักษา credential; ชุดของคนละ Server ปฏิเสธการลงทับเพื่อป้องกันเปลี่ยน CA โดยไม่ได้ย้ายสถานี ถอน Agent เดิมก่อนย้าย Server การถอน Agent ไม่ลบ CA ที่เพิ่มใน CurrentUser Root

ตัวติดตั้งยังไม่ได้ลงลายเซ็น Authenticode Windows อาจแสดงคำเตือน publisher; เผยแพร่ผ่านช่องทางองค์กรที่เชื่อถือและลงลายเซ็นก่อนกระจายวงกว้าง

## ตรวจสอบ

```powershell
installer/dist/payload/python/python.exe scripts/test-installer.py
powershell -NoProfile -File scripts/test-operations.ps1
```

Smoke test ใช้ DB/process/โฟลเดอร์ชั่วคราวแยก ตรวจ migration ซ้ำ, HTTPS ที่ตรวจ CA, เว็บ, บัญชี Admin, auto-enrollment, token ผิด, ชื่อเครื่องชน, สถานีถูกปิด, repair และ compiler ที่สร้าง Client Setup จริง ไม่แก้ trust store, firewall หรือ Scheduled Task ของเครื่องทดสอบ ยังต้องทดสอบ GUI Setup, UAC, reboot และ Client บน Windows VM/เครื่องจริงก่อนกระจาย
