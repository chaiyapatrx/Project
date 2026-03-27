from database import SessionLocal
import models
from datetime import datetime, timedelta

db = SessionLocal()
comps = db.query(models.Computer).filter(models.Computer.name.like('COM-%')).all()

if comps:
    for c in comps:
        c.is_online = True
        c.status = "available"
        c.last_heartbeat = datetime.utcnow() + timedelta(days=3650) # +10 years
    db.commit()
    print(f"✅ สำเร็จ: อัปเดตสถานะ {len(comps)} เครื่อง เป็น 'Online' เรียบร้อยแล้ว (ตั้งค่าให้ออนไลน์ถาวรสำหรับ Mock)")
else:
    print("❌ ไม่พบเครื่อง Mock 50 เครื่องในระบบ โปรดรัน init_db.py ก่อน")

db.close()
