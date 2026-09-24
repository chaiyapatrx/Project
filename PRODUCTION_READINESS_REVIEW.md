# ผลประเมินความพร้อมสำหรับทดลองใช้แบบจำกัด

วันที่ประเมิน: 24 กันยายน 2026

## ความสมบูรณ์ปัจจุบัน: 69%

คะแนนนี้เป็นการประเมินความพร้อมจากโค้ด เอกสาร และผลตรวจที่รันได้ใน workspace ไม่ใช่เปอร์เซ็นต์ test coverage หรือการรับรองว่าไม่มีบั๊ก/ช่องโหว่

| ด้าน | คะแนน | น้ำหนัก | คะแนนถ่วงน้ำหนัก |
|---|---:|---:|---:|
| Workflow หลัก | 82/100 | 25% | 20.5 |
| ความปลอดภัย | 76/100 | 25% | 19.0 |
| ความถูกต้องและความทนทานของข้อมูล | 70/100 | 20% | 14.0 |
| การติดตั้งและการดูแลระบบ | 53/100 | 20% | 10.6 |
| หลักฐานทดสอบและการปล่อยระบบ | 48/100 | 10% | 4.8 |
| **รวม** |  | **100%** | **68.9 ≈ 69%** |

โค้ดพร้อมสำหรับเริ่ม **pilot แบบจำกัดบนเครื่องทดสอบและเครือข่ายที่ควบคุมได้** หลังทำรายการ P0 ด้านล่าง แต่ยังไม่พร้อมเปิดใช้งานผ่านอินเทอร์เน็ตหรือใช้กับผู้ใช้จริงวงกว้าง การตั้งโดเมน/TLS ถูกพักไว้ตามที่ตกลงกัน

## งานที่ทำและตรวจแล้ว

- Backend ตรวจค่า config สำคัญ, จำกัด rate/body size, เพิ่ม security headers, readiness/liveness, ปิด self-registration โดยปริยาย และจำกัด HTTP ที่ไม่เข้ารหัสให้ bind เฉพาะ loopback
- Agent ใช้ secret แยกต่อเครื่อง, เก็บ hash ฝั่ง server, ตรวจลายเซ็นและอายุคำสั่ง, จำกัดคำสั่งตาม role และไม่ส่ง hash ออก API; การออก/หมุน secret ทำได้โดย admin
- เพิ่ม audit events สำหรับการเปลี่ยนแปลงสำคัญ, ป้องกันการลดสิทธิ์/ปิดบัญชี admin คนสุดท้าย และใช้ transaction/row lock ในเส้นทาง booking ที่มีการแข่งขันกัน
- แก้ flow หน้าเว็บและตาราง admin ที่ทำงานไม่ตรงกับ API, ตัดปุ่ม/คำสั่งที่ backend ไม่รองรับ, แก้ CSV formula injection และแก้ lint
- `LOCK`/`UNLOCK` ของ agent เปลี่ยนเป็น overlay เต็มจอ: `LOCK` แสดงและ `UNLOCK` ซ่อน overlay; agent ต้องรันใน interactive Windows user session
- อัปเดต Go dependencies ที่มี advisory ซึ่งแก้ได้; `backend-go` กำหนด Go 1.26 ขึ้นไป
- ปรับ README, ตัวอย่าง environment และสคริปต์ migration ให้ตรงกับการตั้งค่า local trial โดยไม่ใส่ secret จริง

## ผลตรวจที่รันได้

- Backend: `go test ./...` ผ่าน (3 tests ใน 9 packages), `go vet ./...` ผ่าน
- Agent: `go test ./...` และ `go vet ./...` ผ่าน; ไม่มี test cases ใน client package แต่ compile ผ่านทั้ง Windows 386 และ Windows amd64
- ไม่ได้ผลจาก Go race detector เพราะ environment นี้เป็น Windows/386 ซึ่งไม่รองรับ `-race`
- Frontend: `npm run lint`, `npm run build` และ `npm audit --audit-level=high` ผ่าน; audit พบ 0 vulnerabilities
- Go: `govulncheck ./...` ไม่พบช่องโหว่ที่โค้ดเรียกถึง แต่พบ module-level advisory หนึ่งรายการ คือ [GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932) ใน `golang.org/x/crypto/openpgp` ซึ่งเป็น package ที่เลิกดูแลและโค้ด AUCC ไม่ได้ import; advisory นี้ไม่มีเวอร์ชันที่แก้แล้ว
- Migration script ผ่าน Python compile check และ self-check ของ CSV ผ่าน

การตรวจนี้ไม่ได้เชื่อมต่อฐานข้อมูลจริง, ไม่ได้รัน migration กับข้อมูล staging, ไม่ได้ทดสอบ agent บนเครื่องสถานีจริง และไม่ได้ทำ penetration test

## ต้องทำก่อนทดลองกับเครื่อง/ผู้ใช้จริง

### P0 — ก่อนเริ่ม pilot

1. **หมุน credential ของ agent ที่เคยปรากฏใน tool output** ผ่านหน้า Admin แล้วอัปเดต config ของเครื่องนั้นก่อนเชื่อมต่ออีกครั้ง ห้ามนำ credential เก่ากลับมาใช้
2. ยืนยันว่า agent ทำงานใน session ของผู้ใช้ Windows ที่ล็อกอินอยู่ ไม่ใช่ Windows service/session 0; ทดลอง `LOCK`, `UNLOCK`, หมดเวลา และยกเลิก booking บนเครื่องทดสอบหนึ่งเครื่อง
3. Overlay นี้เป็นการบังหน้าจอระดับแอป ไม่ใช่ Windows secure lock ผู้ใช้ที่มีสิทธิ์บนเครื่องยังอาจข้ามหรือปิด agent ได้ หากต้องการบังคับ kiosk ให้ตั้ง Assigned Access/GPO และบัญชี Windows แบบจำกัดสิทธิ์แยกต่างหาก
4. จำกัด NTFS ACL ของ `client-go/config.json` ให้เฉพาะบัญชีที่รัน agent และผู้ดูแลระบบอ่านได้ เพราะ agent ต้องเก็บ station secret ไว้ในเครื่อง

### P1 — ก่อนขยาย pilot

1. สำรองฐานข้อมูล, ทดลอง `migrations/apply_migrations.py` กับสำเนา staging และทดสอบ restore ก่อนใช้กับฐานข้อมูลที่มีข้อมูลเดิม
2. ทดสอบ end-to-end booking, ต่อเวลา, ยกเลิก, หมดเวลา, agent หลุด/ต่อใหม่ และการเปลี่ยนสถานะเครื่อง โดยตรวจทั้ง DB, audit log และหน้าจอ agent
3. กำหนด HTTPS/WSS, allowed origins, cookie secure และ trusted proxy เมื่อได้โดเมน; ห้ามเปิด local HTTP/WS configuration ออกอินเทอร์เน็ต
4. จัดขั้นตอนติดตั้ง/เริ่ม agent ใน interactive session, สำรอง config อย่างปลอดภัย, อัปเดตและ rollback เวอร์ชัน

### P2 — ก่อนใช้งานวงกว้าง

- เพิ่ม monitoring/alert, log retention, ผู้รับผิดชอบ incident และการทดสอบ restore ตามรอบ
- สร้าง release/installer ที่ตรวจสอบไฟล์ได้ และทดสอบบน Windows รุ่น/นโยบายของสถานีจริง
- ตรวจหน้ารายงาน/analytics ที่ยังเป็นข้อมูลตัวอย่างก่อนนำตัวเลขไปใช้ตัดสินใจ

## ขอบเขตและความเสี่ยงที่ยังเหลือ

- ไม่มีผลทดสอบ integration กับ MySQL จริงหรือเครื่อง agent จริง จึงยังยืนยันพฤติกรรม deployment และ race ภายใต้โหลดไม่ได้
- migration เปลี่ยน schema ได้ แต่ยังไม่ได้รันกับฐานข้อมูลที่ใช้งานจริง; backup/restore และการกลับเวอร์ชันยังต้องพิสูจน์บน staging
- agent ไม่ได้ติดตั้งหรือกำหนด autostart ให้เครื่องสถานีโดยอัตโนมัติในงานนี้
- ผล `govulncheck` แยกชัดเจนระหว่าง 0 ช่องโหว่ที่เรียกถึง กับ advisory ที่อยู่ใน package ซึ่งไม่ได้ใช้; ไม่ควรตีความว่า dependency graph ไม่มี advisory ทุกชนิด
- ไม่ควรใช้ 69% เป็นใบรับรอง production readiness คะแนนจะเปลี่ยนหลัง pilot, restore drill, TLS/deployment และการทดสอบเครื่องจริง
