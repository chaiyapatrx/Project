
# init_db.py
from database import SessionLocal, engine
import models
from passlib.context import CryptContext

# สร้างตารางให้ชัวร์ (เผื่อยังไม่สร้าง)
models.Base.metadata.create_all(bind=engine)

db = SessionLocal()
pwd_context = CryptContext(schemes=["bcrypt"], deprecated="auto")

def create_admin():
    # เช็คก่อนว่ามี Admin หรือยัง
    existing_admin = db.query(models.User).filter(models.User.username == "admin").first()
    if existing_admin:
        print("User 'admin' มีอยู่แล้วครับ")
    else:
        # สร้าง Admin คนแรก
        admin_user = models.User(
            username="admin",
            password_hash=pwd_context.hash("1234"), # รหัสผ่านคือ 1234
            full_name="Super Admin",
            role="admin",  # สำคัญตรงนี้
            department="IT Center",
            user_type="staff"
        )
        
        db.add(admin_user)
        db.commit()
        print("✅ สร้าง Admin สำเร็จ! (User: admin / Pass: 1234)")

    # สร้าง Staff
    existing_staff = db.query(models.User).filter(models.User.username == "staff01").first()
    if not existing_staff:
        staff_user = models.User(
            username="staff01",
            password_hash=pwd_context.hash("1234"),
            full_name="Staff Member",
            role="staff",
            department="Library",
            user_type="staff"
        )
        db.add(staff_user)
        db.commit()
        print("✅ สร้าง Staff สำเร็จ! (User: staff01 / Pass: 1234)")
    
    # สร้าง User ทั่วไป
    existing_user = db.query(models.User).filter(models.User.username == "user01").first()
    if not existing_user:
        normal_user = models.User(
            username="user01",
            password_hash=pwd_context.hash("1234"),
            full_name="Normal User",
            role="user",
            department="General",
            user_type="user"
        )
        db.add(normal_user)
        db.commit()
        print("✅ สร้าง User สำเร็จ! (User: user01 / Pass: 1234)")

    # Mock ข้อมูลเครื่องคอมพิวเตอร์ 50 เครื่อง
    existing_computers = db.query(models.Computer).count()
    if existing_computers < 50:
        for i in range(1, 51):
            comp_name = f"COM-{i:02d}"
            existing_comp = db.query(models.Computer).filter(models.Computer.name == comp_name).first()
            if not existing_comp:
                from datetime import datetime, timedelta
                new_comp = models.Computer(
                    name=comp_name,
                    status="available",
                    is_online=True,
                    last_heartbeat=datetime.utcnow() + timedelta(days=3650)
                )
                db.add(new_comp)
        db.commit()
        print("✅ สร้างข้อมูลคอมพิวเตอร์ 50 เครื่องสำเร็จ!")

if __name__ == "__main__":
    create_admin()