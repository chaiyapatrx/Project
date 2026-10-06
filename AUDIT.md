# ผลตรวจและปรับแก้ AUCC

ตรวจวันที่ **6 ตุลาคม 2026** จากไฟล์ปัจจุบันใน workspace รวมงานแก้เดิมที่ยังไม่ commit แยก agent ตรวจ Backend, Client และ Frontend; ผู้ประสานตรวจ migration, launcher, backup, firewall, เอกสาร และตรวจผลข้ามส่วน

รายงานนี้บันทึกสิ่งที่ตรวจและหลักฐานที่รันจริง ไม่ใช่คำรับรองว่าไม่มีบั๊ก/ช่องโหว่ทั้งหมด ไม่ใช้เปอร์เซ็นต์ความพร้อมที่ไม่มี benchmark รองรับ

## รอบตรวจเพิ่มเติมสำหรับ production และ SADMIN

**ข้อความตัวอย่างใน README เดิมเป็นความเสี่ยงจริงหากคัดลอกมาใช้:** ชื่อตัวแปร SADMIN ไม่ใช่ backdoor แต่โค้ดเดิมตรวจเพียงความยาว จึงรับ password ตัวอย่างบางค่าได้ และเปลี่ยนชื่อ bootstrap แล้วสร้าง Admin เพิ่มได้ แก้ดังนี้:

- `BOOTSTRAP_ADMIN=false` โดยค่าเริ่มต้น เปิดเฉพาะ setup ครั้งแรก; ปิดแล้ว runtime ไม่โหลด SADMIN credentials เข้า config
- ปฏิเสธชื่อ/รหัสตัวอย่างที่แจกในคู่มือ, รหัสซ้ำอักษรเดียว และรหัส bootstrap สั้นกว่า 15 ตัวอักษรหรือยาวกว่า 72 UTF-8 bytes การตรวจนี้ไม่ใช่ตัววัด entropy ให้สร้างรหัสสุ่มเฉพาะระบบ
- ใช้ durable marker `admin_bootstrap_completed` และ transaction/unique settings key ป้องกัน concurrent bootstrap และการสร้างกลับหลังลบ Admin แม้ DB ใช้ READ COMMITTED
- ตรวจ SQL errors แล้วหยุด bootstrap แบบ fail closed; ไม่เปลี่ยน password/role ของบัญชีเดิม ไม่พิมพ์ username/password หรือ DB error detail ใน bootstrap log; ล้าง bootstrap password จาก runtime config หลังใช้
- เปลี่ยน cookie config ให้ parse แบบ strict และบังคับ secure cookie เมื่อเปิด HTTPS
- ใช้ SafeRecovery แทน Gin recovery ที่อาจพิมพ์ Cookie/X-Agent-Secret และเปลี่ยน access logger ให้บันทึก route template เท่านั้น ไม่พิมพ์ query/path parameters/headers/body/panic payload ที่มีค่าจากผู้ใช้
- เพิ่ม guarded password update กันการเปลี่ยนรหัสทับ reset/logout/deactivation/role change และ token revocation
- รหัสผ่านใหม่ทุก API/หน้าเว็บต้องอย่างน้อย 15 Unicode characters และไม่เกิน 72 UTF-8 bytes; ตรวจ login ไม่ให้ bcrypt ตัด suffix หลัง byte 72; รหัสเดิมยังใช้ได้จนผู้ดูแลเปลี่ยนตามขั้นตอน
- เพิ่มการจำกัดการเดารหัสบัญชี 10 ครั้ง/นาทีร่วม browser/station/password confirmation โดย user ID ไม่ใช่ IP อย่างเดียว; dummy bcrypt เมื่อชื่อไม่มี ลด timing difference
- Backend ส่ง nonce ที่สุ่มและ signed ใน command Data; Agent ปฏิเสธ replay ข้าม reconnect
- Agent 1.0.9 จำกัด auto-update ให้ trusted HTTPS แม้ localhost, ปฏิเสธ userinfo ใน URL และไม่ log raw config URL
- Agent config ใช้ protected Windows DACL บัญชีสถานี/SYSTEM/Administrators; หากตั้ง ACL ไม่ผ่านให้หยุด
- API 401 ล้างข้อมูลผู้ใช้ใน frontend; เปลี่ยน session ข้ามแท็บแล้ว revalidate กับ server
- Settings API ส่งเฉพาะ allowlisted settings ไม่เปิดเผย bootstrap marker/internal values

### สิทธิ์ไฟล์บนเครื่องนี้

พบ `.env`, `.env.before-xampp`, backups และ deployment folder เดิม inherit สิทธิ์อ่าน/แก้ไขจาก Users/Authenticated Users จริง จำกัดสิทธิ์เรียบร้อยด้วย `scripts/protect-secrets.ps1` และ helper ครอบคลุมโฟลเดอร์โปรเจกต์, ไฟล์ลับ, TLS private key และ config สถานีที่ใช้พัฒนา ตรวจซ้ำ project/EXE/env พบ broad allow entries = 0; secret ACL protected = true ไม่อ่านหรือพิมพ์ค่าลับในรายงาน

**ยังมี bootstrap password เดิมใน `.env` ของเครื่องนี้** แต่ ACL จำกัดและ bootstrap ปิดอยู่ ไม่ลบทิ้งอัตโนมัติเพราะอาจเป็นสำเนาสุดท้ายที่ผู้ดูแลใช้กู้บัญชี หลังเก็บรหัสในที่เก็บของผู้ดูแลอย่างปลอดภัย ต้องนำ SADMIN credentials ออกจาก config/สำเนาเก่า ไฟล์ลับทั้งหมดต้องอยู่ private deployment directory และผู้ดูแลเครื่องยังเป็น trusted principal

### หลักฐานเพิ่มเติม

- Backend suite ล่าสุด 88 tests ผ่าน; vet ผ่าน; สร้าง production-amd64 EXE แล้ว; security regressions ก่อนเพิ่ม access-logger check รันซ้ำสองรอบผ่าน 152 checks
- Client production-amd64 suite 21 tests ผ่าน; `govulncheck` ไม่พบช่องโหว่; installer/EXE รุ่น 1.0.9 ล่าสุด
- Frontend lint/native regressions/build ผ่าน รวม policy ภาษาไทย/emoji, expired session และ cross-tab handling
- ตรวจ Git history ที่ reachable ทั้งหมด: repo ไม่ shallow, 3 commits, 225 commit/path entries, 165 unique blobs; เทียบค่าลับปัจจุบัน 2 ค่าไม่พบ exposure พบ credential fixture ใน test เก่า 1 จุด ไม่ใช่ค่าลับปัจจุบัน ไม่ rewrite history
- ตรวจ frontend build เทียบค่าลับปัจจุบันที่ตรวจ 2 ค่า ไม่พบ matches และไม่มี sourcemaps การตรวจแบบนี้ไม่พิสูจน์ secret leak ทุกประเภท
- Backup หลังเพิ่ม ACL protection สำเร็จอีกครั้ง โดยไม่เปลี่ยนข้อมูล DB หรือเปิด Agent overlay

**ยังไม่ควรรับรอง production ว่าไม่มีช่องโหว่:** ต้องผ่าน live end-to-end, concurrent MySQL และ restore/pentest; ไม่มี MFA; throttles เป็น per-process; replay cache ไม่ข้ามการ restart process; updater เชื่อถือ server/Admin release และยังไม่มี independent publisher signature; overlay ไม่ใช่ OS secure lock; ยังมี 5 high advisories ใน development toolchain ที่ upstream ไม่มี patch ดูรายละเอียดด้านล่าง แนวทางอ้างอิง [OWASP Authentication](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html) และ [Secrets Management](https://cheatsheetseries.owasp.org/cheatsheets/Secrets_Management_Cheat_Sheet.html)

## ขอบเขต

- Backend: อ่านทุกไฟล์ production ใน `cmd/server`, `internal/auth`, `config`, `database`, `handler`, `hub`, `middleware`, `models`, `worker`; ไล่ routes, role, JWT/CSRF, station credentials, booking/session transactions และการส่งคำสั่ง
- Client: อ่าน Agent, WebView/HTML, autostart, config, enrollment, network, update downloader, updater, BAT build/runner และ Inno Setup
- Frontend: อ่าน source และ config ทั้งหมด; ตรวจการเรียก API, auth restore/logout, storage, WebSocket lifecycle, CSV, role routes และการแสดงข้อมูลรายงาน
- Migration: numbered up/down SQL, ledger, SQL parser, TLS, การ resume หลัง DDL ทำบางส่วน และ configuration parsing
- Windows operations: start/stop/backup/firewall scripts และ command wrappers; ตรวจการใช้ host/port, credentials, file paths และ retention
- Dependency: Go source scan ทั้งสอง module และ npm audit ทั้ง runtime และ development packages

## บั๊กและช่องโหว่ที่แก้

### Backend

| สิ่งที่พบ | ผลกระทบ | การแก้ |
|---|---|---|
| JWT validation ไม่บังคับ expiry | token ที่เซ็นถูกแต่ไม่มี exp อาจผ่าน validation | ใช้ `WithExpirationRequired` และ regression test |
| Monitor socket ตรวจ token ตอนเปิดอย่างเดียว | socket ยังรับข้อมูลหลัง token หมดอายุ | timer ปิด socket เมื่อถึง exp มีการทดสอบ WebSocket ผ่าน network |
| สถานีถูก disabled ระหว่าง handshake | reconnect อาจรับสถานีที่ถูกปิดแล้ว | ตรวจสถานะซ้ำภายใต้ DB lock และใช้สถานะ DB ใน Hub |
| booking recovery/worker ไม่เห็น walk-in ที่เข้ามาใหม่ | งานเก่าอาจคืนสถานะหรือล็อกทับ session ใหม่ | นับ booking และ usage ที่เปิดก่อน reconcile/expire |
| ต่อเวลาการจองที่หมดอายุแล้ว | ต่อ session ที่ควรสิ้นสุดได้ก่อน worker ทำงาน | ปฏิเสธ extension เมื่อ end_time ผ่านแล้ว |
| update Hub หลังปล่อย transaction lock | extension/cancellation/status รุ่นเก่าอาจทับสถานะใหม่ | รักษาลำดับ DB lock/Hub update |
| logout/expiration เปลี่ยน maintenance เป็น available | เปิดสถานีที่ผู้ดูแลปิดปรับปรุงไว้ | เก็บสถานะ maintenance/disabled เมื่อจบ session |
| HTTP body/DB I/O ไม่มี deadline ครบ | slow client หรือ DB ค้างใช้ทรัพยากรไม่สิ้นสุด | HTTP body 30s; DB connect 5s/read-write 15s |

### Client และ updater

- แก้ BAT ที่ `exit` ทำงานทุกครั้งแม้มีไฟล์ Agent จึงเปิด Agent ไม่ได้
- build/runner ใช้ชื่อ `AUCCAgent.exe` คู่ `AUCCUpdater.exe` และ VERSION เดียวกัน กำหนด Windows amd64 ชัดเจน
- เปลี่ยนชื่อ/ตำแหน่ง EXE ที่ไม่รองรับ auto-update แล้วรายงาน error โดยไม่ปิด Agent ไปเอง
- ย้ายการเปลี่ยน auth mode ของ WebView ไป UI thread
- จำกัดข้อความ WebSocket 64 KiB และ write deadline 5 วินาที ลดการรับข้อมูลใหญ่หรือค้าง
- updater ตรวจรุ่น EXE มี timeout 10 วินาที และปฏิเสธ staged path ที่ตรงกับ EXE ปัจจุบัน
- อัปเดต `golang.org/x/sys` เป็น `v0.44.0` ลบ advisory ที่ scanner พบใน dependency
- เพิ่ม regression สำหรับ HMAC ที่ถูกแก้ไข/หมดอายุ/อยู่อนาคต, oversized commands, hung preflight และ BAT runner ด้วย GUI stub ที่ไม่ล็อกเครื่อง
- เพิ่มรุ่นเป็น **1.0.9** และสร้าง Agent, updater และ installer ล่าสุด

### Frontend

- WebSocket cleanup ผูกกับแต่ละ effect ป้องกัน socket เก่า reconnect หรือส่งข้อมูลหลัง logout/เปลี่ยนผู้ใช้/StrictMode
- ใช้ refs ตาม React lifecycle และ retry เมื่อสร้าง socket ไม่ผ่าน
- API helper ปฏิเสธ origin อื่นก่อนส่ง cookies/CSRF; อ่าน CSRF cookie ปัจจุบันลดปัญหาหลายแท็บ
- auth restore ต้องยืนยันกับ backend; logout ที่ backend ล้มเหลวไม่แสดงว่าจบ session สำเร็จ
- dashboard/login ไม่พังเมื่อ browser ปิด localStorage
- Staff logout ส่ง `LOGOUT` และนับ session ที่จบแม้เครื่อง offline
- Daily Report ใช้เฉพาะ session วันนี้ที่จบแล้ว ลบตัวเลข peak hours ตัวอย่าง; Analytics แสดง refresh failure
- คืน CSV object URL หลังดาวน์โหลด และตรวจ CSV injection ใน regression
- ลบปุ่ม Filter/Export ที่ไม่ทำงาน, Vite demo CSS และ Electron/external portal branches ที่ไม่มี package/preload/IPC implementation ในโปรเจกต์ปัจจุบัน
- อัปเดต `source-map-js` เป็น 1.2.2 และ override `postcss-selector-parser` เป็น 7.1.6 เพื่อลบ advisory ที่มี patch พร้อมแล้ว ตรวจ API callers และ CSS ก่อน/หลังเหมือนกันทุก byte

### Migration และสคริปต์

- ลบ `init_database.sql` ที่ซ้ำกับ numbered migrations; runner ใช้ลำดับ SQL และ ledger ชุดเดียว
- เพิ่ม connection advisory lock ป้องกัน runner สองตัวแก้ schema พร้อมกัน
- migration 000006 resume ได้ถ้าคอลัมน์สร้างแล้วแต่ index/ledger ยังไม่เสร็จ คงกลไก resume 000007 ไว้
- การอ่าน `.env` ใน migration/operations รองรับ quoted inline comments, escape และ interpolation ของค่าก่อนหน้า ให้ตรงกับรูปแบบ godotenv ที่ backend ใช้
- backup เดิมมี PowerShell syntax error; แก้พร้อมใช้ DB config จริง, validate port/retention/name และไม่ใส่รหัสผ่านใน process command line
- mysqldump เขียนไฟล์เองเพื่อรักษา encoding; ไฟล์ไม่เสร็จใช้ `.partial`; ลบ backup เก่าเฉพาะหลัง backup ใหม่สำเร็จ และตรวจโฟลเดอร์เป้าหมาย
- stop จำกัด backend ตาม EXE path ของโปรเจกต์; เลิกบังคับ kill MySQL; ปฏิเสธ shutdown remote DB
- start ใช้ DB host/port ที่ตั้งจริง และส่ง port ให้ mysqld เมื่อเปิด XAMPP; CMD ใช้พาธของตัวเองแทนไดรฟ์ที่ hardcode
- firewall ต้องระบุ subnet และใช้ Private profile; รันซ้ำปรับกฎเดิมได้
- เพิ่ม `migrations/requirements.txt`, ignored backups และ regression scripts สำหรับ operations

## หลักฐานการตรวจ

| ตรวจ | ผล |
|---|---|
| Backend `go test ./...` | ผ่าน 88 tests ใน 9 packages |
| Backend `go vet ./...` | ผ่าน |
| Backend Windows amd64 build | ผ่าน สร้าง `backend-go/AUCCServer.exe` ล่าสุด |
| Client Windows amd64 `go test ./...` | ผ่าน 21 tests ใน 2 packages |
| Client `go vet -unsafeptr=false ./...` | ผ่าน analyzers ที่เปิดใช้ |
| Client full `go vet ./...` | ยังแจ้ง Win32 `WM_DRAWITEM` LPARAM cast เป็น pointer ใน callback |
| Client installer build | ผ่าน `AUCCAgentSetup-1.0.9.exe`; EXE `--version` ตอบ 1.0.9 |
| Frontend lint/native regression/build | ผ่าน `npm run lint`, `npm test`, `npm run build` |
| Frontend parser override CSS comparison | SHA-256 ก่อน/หลัง `0d98287b8fa9e1da7f81c70a470aa67c78108d9bd9faca73207fbe07ac6f8825` |
| Python migration tests | ผ่าน 7 tests |
| PowerShell operations tests/parser | ผ่าน ไม่เปลี่ยน DB/process/firewall ใน regression |
| Backup กับ DB ที่ตั้งอยู่จริง | mysqldump สำเร็จ สร้าง SQL ใน `backups` ใช้ retention 36500 วันเพื่อไม่ลบ backup เดิม |
| `git diff --check` | ผ่าน |

ไฟล์ทดสอบที่เพิ่ม/ขยายอยู่ร่วมกับแต่ละส่วน คำสั่งรันซ้ำดู README ผล backup ยืนยันการ dump สำเร็จ **ยังไม่ใช่ผล restore** และไม่มีการ apply migration ใหม่ลงฐานจริงในรอบตรวจนี้

## Dependency และความเสี่ยงค้าง

### ผล scan ล่าสุด

- Client `govulncheck`: **No vulnerabilities found** หลังอัปเดต x/sys
- Backend `govulncheck`: ไม่พบช่องโหว่ใน code paths/imported packages ที่ใช้งาน ยังมี module advisory ใน `golang.org/x/crypto/openpgp` ซึ่งระบบไม่ import; backend ใช้ bcrypt ไม่มี patched version สำหรับ [GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932)
- `npm audit --omit=dev`: **0 vulnerabilities** ใน production dependencies
- full npm audit: **5 high, 0 moderate** เป็น `braces`, `chokidar`, `micromatch`, `fast-glob`, `tailwindcss` ในเครื่องมือพัฒนา/build ทั้ง 5 รายการสืบจาก braces recursion advisory ไม่ใช่ 5 ช่องโหว่แยกกัน ปัจจุบัน [braces advisory](https://github.com/advisories/GHSA-vfj7-8cjw-p6xm) ระบุไม่มี patched version
- การลบ dependency สายนี้ทั้งหมดต้องเปลี่ยน Tailwind build integration และตรวจความเข้ากันได้ของ UI ไม่ใช้ `npm audit fix --force` โดยไม่มีการตรวจ UI ในงานนี้ อย่าให้ build pipeline รับ glob/config จากผู้ใช้ที่ไม่เชื่อถือ และอย่าเปิด Vite dev server สู่เครือข่ายจริง

### สิ่งที่ยังต้องพิสูจน์ก่อนเปิดใช้งานจริง

1. **Station end-to-end:** ยังไม่ได้เปิด overlay หรือติดตั้ง GUI บน client จริงในรอบนี้ ต้องทดสอบ booking/walk-in/timeout/disconnect/restart/rollout/rollback บน Windows policy จริง
2. **DB concurrency:** tests ครอบคลุม logic/SQL mocks แต่ยังไม่มี load/concurrent integration บนฐาน staging
3. **Restore/recovery:** dump ผ่านแล้ว แต่ต้องทดลอง restore staging พร้อม release storage และการกลับมา online
4. **Race detector:** host Go เดิมใช้ windows/386 และ CGO=0; การ build ใหม่เป็น amd64 ไม่ทำให้ race detector ใช้ได้ทันที ยังต้องมี CGO/compiler ที่รองรับ
5. **Windows ABI vet:** pointer จาก LPARAM เป็นรูปแบบ callback ของ Win32 ที่ใช้อยู่ จึงคงไว้ รายงาน full vet ตามจริง ไม่แปลงว่า warnings ทั้งหมดหายแล้ว
6. **ขอบเขต Agent:** overlay ไม่ใช่ OS secure lock การปิด process/เปลี่ยน Windows session อาจข้ามได้ ใช้บัญชีสิทธิ์จำกัดและนโยบายของห้องปฏิบัติการ
7. **ปริมาณข้อมูล/รายงาน:** usage-history API จำกัด 100 รายการ และไม่มี full-history pagination จึงอย่าใช้รายงานจากรายการนี้แทนประวัติทั้งฐานเมื่อข้อมูลเกินขอบเขต
8. **Capacity:** ไม่มี benchmark จำนวนสถานีหรือ latency ไม่รับรองค่าความจุจากแผนเก่า

## การเคลียร์ไฟล์

ลบไฟล์ที่ยืนยันว่าเก่า/ซ้ำ/ไม่ใช้งาน:

- `PRODUCTION_MIGRATION_PLAN.md` — แผนย้อนหลังที่ระบุสถาปัตยกรรม/ความจุไม่ตรงปัจจุบัน
- `PRODUCTION_READINESS_REVIEW.md` — รายงานเก่าและเปอร์เซ็นต์ประเมิน แทนด้วยรายงานนี้
- `codex_check_admin_api.py` — สคริปต์เฉพาะเครื่องที่ hardcode IP, station ID และเลขรุ่น และมีคำสั่ง publish/logout
- `migrations/init_database.sql` — combined schema ซ้ำและไม่ครอบคลุม upgrade เท่า numbered migrations
- `frontend/src/App.css` — Vite demo ไม่มี reference
- Electron/external portal branches ที่ไม่มี implementation ปัจจุบัน
- binaries เก่า `backend-go/AUCCServer.new.exe`, `AUCCServer.previous.exe`, `server.exe`, `client-go/agent.exe`
- Agent Setup 1.0.0–1.0.8 ใน `client-go/dist` หลัง build 1.0.9 สำเร็จ

เก็บ source, regression tests, numbered up/down SQL, lockfiles, config examples และ public CA ที่ใช้ build ไว้ ไม่ลบ `.env`, config สถานี, certificate/private keys, backup หรือ server-side release storage ที่ยังใช้กู้คืนระบบ งานที่ผู้ใช้แก้ค้างไว้ก่อนเริ่มยังอยู่ ไม่ reset workspace

เอกสารปัจจุบัน: [README.md](README.md) อธิบาย workflow/ติดตั้ง/ดูแล/แก้ปัญหา และ [LAN_SETUP.md](LAN_SETUP.md) สำหรับ certificate/HTTPS/WSS/firewall/Agent ใน LAN
