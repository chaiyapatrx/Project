from fastapi import FastAPI, Depends, HTTPException, status
from fastapi.security import OAuth2PasswordBearer, OAuth2PasswordRequestForm
from fastapi.middleware.cors import CORSMiddleware
from sqlalchemy.orm import Session
from passlib.context import CryptContext
from datetime import timedelta, datetime
import random
import string
import models
from database import SessionLocal, engine
from pydantic import BaseModel
from typing import List, Optional

# สร้างตารางใน DB
models.Base.metadata.create_all(bind=engine)

app = FastAPI()

# --- CORS ---
origins = [
    "http://localhost:5173", 
    "http://localhost:3000",
    "*", # Allow all for network testing
]

app.add_middleware(
    CORSMiddleware,
    allow_origins=origins,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# --- Config ---
SECRET_KEY = "supersecretkey"
ALGORITHM = "HS256"
pwd_context = CryptContext(schemes=["bcrypt"], deprecated="auto")
oauth2_scheme = OAuth2PasswordBearer(tokenUrl="token")

# --- Dependency ---
def get_db():
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()

# --- Utility ---
def verify_password(plain_password, hashed_password):
    return pwd_context.verify(plain_password, hashed_password)

def get_password_hash(password):
    return pwd_context.hash(password)

def get_current_user(token: str = Depends(oauth2_scheme), db: Session = Depends(get_db)):
    user = db.query(models.User).filter(models.User.username == token).first()
    if not user:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid authentication credentials",
            headers={"WWW-Authenticate": "Bearer"},
        )
    return user

def generate_access_code():
    return ''.join(random.choices(string.ascii_uppercase + string.digits, k=6))

# --- Pydantic Models ---
class Token(BaseModel):
    access_token: str
    token_type: str

class UserCreate(BaseModel):
    username: str
    password: str
    full_name: str
    role: str = "user" # ค่าเริ่มต้นเป็น user

class UserResponse(BaseModel):
    id: int
    username: str
    full_name: str
    role: str
    department: Optional[str] = None
    user_type: Optional[str] = None

    class Config:
        orm_mode = True

class ComputerCreate(BaseModel):
    name: str

class BookingCreate(BaseModel):
    computer_id: int
    start_time: datetime
    end_time: datetime

class UserAdminUpdate(BaseModel):
    role: Optional[str] = None
    department: Optional[str] = None

# --- API Endpoints ---

# 0. สมัครสมาชิก (สำคัญ! ต้องใช้สร้าง User คนแรก)
@app.post("/register")
def register_user(user: UserCreate, db: Session = Depends(get_db)):
    # เช็คว่ามี username นี้หรือยัง
    db_user = db.query(models.User).filter(models.User.username == user.username).first()
    if db_user:
        raise HTTPException(status_code=400, detail="Username already registered")
    
    # สร้าง User ใหม่และ Hash Password
    hashed_password = get_password_hash(user.password)
    new_user = models.User(
        username=user.username,
        password_hash=hashed_password,
        full_name=user.full_name,
        role=user.role
    )
    db.add(new_user)
    db.commit()
    db.refresh(new_user)
    return {"message": "User created successfully", "username": new_user.username}

# 1. Login
@app.post("/token", response_model=Token)
async def login_for_access_token(form_data: OAuth2PasswordRequestForm = Depends(), db: Session = Depends(get_db)):
    user = db.query(models.User).filter(models.User.username == form_data.username).first()
    if not user or not verify_password(form_data.password, user.password_hash):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Incorrect username or password",
            headers={"WWW-Authenticate": "Bearer"},
        )
    return {"access_token": user.username, "token_type": "bearer"}

# 2. ดูข้อมูลตัวเอง
@app.get("/users/me")
async def read_users_me(current_user: models.User = Depends(get_current_user)):
    return {
        "id": current_user.id,
        "username": current_user.username,
        "role": current_user.role,
        "full_name": current_user.full_name
    }

# NEW: ดูผู้ใช้ทั้งหมด (สำหรับ Admin)
@app.get("/users", response_model=List[UserResponse])
def get_all_users(user: models.User = Depends(get_current_user), db: Session = Depends(get_db)):
    if user.role != "admin":
        raise HTTPException(status_code=403, detail="Not authorized")
    return db.query(models.User).all()

# 3. จัดการคอมพิวเตอร์ (ดูทั้งหมด)
@app.get("/computers")
def get_computers(db: Session = Depends(get_db)):
    return db.query(models.Computer).all()

# 4. เพิ่มคอมพิวเตอร์
@app.post("/computers")
def create_computer(computer: ComputerCreate, user: models.User = Depends(get_current_user), db: Session = Depends(get_db)):
    if user.role not in ["admin", "staff"]:
        raise HTTPException(status_code=403, detail="ไม่มีสิทธิ์เพิ่มคอมพิวเตอร์")
    
    db_comp = models.Computer(name=computer.name, status="available")
    db.add(db_comp)
    db.commit()
    db.refresh(db_comp)
    return db_comp

# 5. จองเครื่อง
@app.post("/bookings")
def create_booking(booking: BookingCreate, user: models.User = Depends(get_current_user), db: Session = Depends(get_db)):
    # เช็คว่าผู้ใช้มีการจองที่ active อยู่แล้วหรือไม่ (จำกัด 1 เครื่องต่อ 1 คน)
    now = datetime.utcnow()
    existing_user_booking = db.query(models.Booking).filter(
        models.Booking.user_id == user.id,
        models.Booking.status == "confirmed",
        models.Booking.end_time > now
    ).first()

    if existing_user_booking:
        raise HTTPException(status_code=400, detail="คุณมีการจองที่กำลังใช้งานอยู่แล้ว (จำกัด 1 เครื่องต่อ 1 คน) กรุณายกเลิกของเดิมก่อน")

    # เช็คเวลาทับซ้อน
    collision = db.query(models.Booking).filter(
        models.Booking.computer_id == booking.computer_id,
        models.Booking.status == "confirmed",
        models.Booking.end_time > booking.start_time,
        models.Booking.start_time < booking.end_time
    ).first()

    if collision:
        raise HTTPException(status_code=400, detail="เครื่องไม่ว่างในช่วงเวลานี้")

    new_booking = models.Booking(
        user_id=user.id,
        computer_id=booking.computer_id,
        start_time=booking.start_time,
        end_time=booking.end_time,
        access_code=generate_access_code(),
        status="confirmed"
    )
    db.add(new_booking)
    
    # Update computer status to occupied
    db_comp = db.query(models.Computer).filter(models.Computer.id == booking.computer_id).first()
    if db_comp:
        db_comp.status = "occupied"
        
    db.commit()
    return {"message": "จองสำเร็จ", "access_code": new_booking.access_code}

# 6. ดูประวัติ (User)
@app.get("/my-bookings")
def get_my_bookings(user: models.User = Depends(get_current_user), db: Session = Depends(get_db)):
    # Load computer relationship to show computer name
    from sqlalchemy.orm import joinedload
    return db.query(models.Booking).options(joinedload(models.Booking.computer)).filter(models.Booking.user_id == user.id).all()

# 7. ยกเลิกการจอง
@app.delete("/bookings/{booking_id}")
def cancel_booking(booking_id: int, user: models.User = Depends(get_current_user), db: Session = Depends(get_db)):
    booking = db.query(models.Booking).filter(
        models.Booking.id == booking_id, 
        models.Booking.user_id == user.id
    ).first()

    if not booking:
        raise HTTPException(status_code=404, detail="Booking not found or not authorized")
    
    if booking.status != "confirmed":
        raise HTTPException(status_code=400, detail="Cannot cancel this booking")
        
    booking.status = "cancelled"
    
    # Update computer status back to available
    db_comp = db.query(models.Computer).filter(models.Computer.id == booking.computer_id).first()
    if db_comp and db_comp.status == "occupied":
        # Check if there are other current confirmed bookings for this computer
        now = datetime.utcnow()
        other_bookings = db.query(models.Booking).filter(
            models.Booking.computer_id == booking.computer_id,
            models.Booking.status == "confirmed",
            models.Booking.id != booking.id,
            models.Booking.end_time > now,
            models.Booking.start_time < now
        ).first()
        
        if not other_bookings:
            db_comp.status = "available"

    db.commit()
    return {"message": "Booking cancelled successfully"}

# --- New Admin Endpoints ---

class ComputerUpdate(BaseModel):
    name: Optional[str] = None
    status: Optional[str] = None
    is_online: Optional[bool] = None

class HeartbeatSchema(BaseModel):
    computer_name: str
    hwid: str 
    is_online: bool

class CommandRequest(BaseModel):
    command: str

@app.post("/api/heartbeat")
def receive_heartbeat(data: HeartbeatSchema, db: Session = Depends(get_db)):
    # Find computer by HWID first (more reliable) or Name
    comp = db.query(models.Computer).filter(models.Computer.hwid == data.hwid).first()
    
    if not comp:
        # Fallback: Find by name (maybe it was created manually without hwid)
        comp = db.query(models.Computer).filter(models.Computer.name == data.computer_name).first()
    
    if not comp:
        # Auto-register
        comp = models.Computer(
            name=data.computer_name, 
            hwid=data.hwid,
            status="available", 
            is_online=data.is_online,
            last_heartbeat=datetime.utcnow()
        )
        db.add(comp)
    else:
        # Update existing
        comp.is_online = data.is_online
        comp.last_heartbeat = datetime.utcnow()
        if not comp.hwid: # Update HWID if missing
            comp.hwid = data.hwid
        
    # Check for pending commands
    commands = db.query(models.ComputerCommand).filter(
        models.ComputerCommand.computer_id == comp.id,
        models.ComputerCommand.status == "pending"
    ).all()
    
    command_list = []
    for cmd in commands:
        command_list.append(cmd.command_type)
        cmd.status = "sent"
        print(f"[Command] Sending {cmd.command_type} to {comp.name}")
        
    db.commit()
    return {"message": "Heartbeat received", "commands": command_list}

# Background Task for Offline Detection
import threading
import time

def check_offline_clients():
    while True:
        try:
            db = SessionLocal()
            # Timeout threshold: 60 seconds
            threshold = datetime.utcnow() - timedelta(seconds=60)
            
            # Find computers that are 'online' but haven't sent heartbeat recently
            offline_comps = db.query(models.Computer).filter(
                models.Computer.is_online == True,
                models.Computer.last_heartbeat < threshold
            ).all()
            
            for comp in offline_comps:
                print(f"[Auto-Offline] Marking {comp.name} as offline.")
                comp.is_online = False
                
            if offline_comps:
                db.commit()
            
            db.close()
        except Exception as e:
            print(f"Error in background task: {e}")
        
        time.sleep(30) # Check every 30s

# Start background thread
threading.Thread(target=check_offline_clients, daemon=True).start()

@app.get("/dashboard/stats")
def get_dashboard_stats(user: models.User = Depends(get_current_user), db: Session = Depends(get_db)):
    if user.role not in ["admin", "staff"]:
        raise HTTPException(status_code=403, detail="Not authorized")
    
    total_users = db.query(models.User).count()
    total_computers = db.query(models.Computer).count()
    active_bookings = db.query(models.Booking).filter(models.Booking.status == "confirmed").count()
    
    # Mock data for charts
    return {
        "total_users": total_users,
        "total_computers": total_computers,
        "active_bookings": active_bookings,
        "todays_bookings": 12  # Mock value
    }

@app.put("/computers/{computer_id}")
def update_computer(computer_id: int, comp: ComputerUpdate, user: models.User = Depends(get_current_user), db: Session = Depends(get_db)):
    if user.role != "admin":
        raise HTTPException(status_code=403, detail="Not authorized")
    
    db_comp = db.query(models.Computer).filter(models.Computer.id == computer_id).first()
    if not db_comp:
        raise HTTPException(status_code=404, detail="Computer not found")
    
    if comp.name: db_comp.name = comp.name
    if comp.status: db_comp.status = comp.status
    if comp.is_online is not None: db_comp.is_online = comp.is_online
    
    db.commit()
    return {"message": "Computer updated"}

@app.delete("/computers/{computer_id}")
def delete_computer(computer_id: int, user: models.User = Depends(get_current_user), db: Session = Depends(get_db)):
    if user.role != "admin":
        raise HTTPException(status_code=403, detail="Not authorized")
    
    db_comp = db.query(models.Computer).filter(models.Computer.id == computer_id).first()
    if not db_comp:
        raise HTTPException(status_code=404, detail="Computer not found")
        
    db.delete(db_comp)
    db.commit()
    return {"message": "Computer deleted"}

@app.post("/api/admin/computers/{computer_id}/command")
def send_command(computer_id: int, request: CommandRequest, user: models.User = Depends(get_current_user), db: Session = Depends(get_db)):
    if user.role not in ["admin", "staff"]:
        raise HTTPException(status_code=403, detail="Not authorized")
        
    comp = db.query(models.Computer).filter(models.Computer.id == computer_id).first()
    if not comp:
        raise HTTPException(status_code=404, detail="Computer not found")
        
    new_cmd = models.ComputerCommand(
        computer_id=comp.id,
        command_type=request.command,
        status="pending"
    )
    db.add(new_cmd)
    db.commit()
    return {"message": f"Command {request.command} queued"}

@app.get("/admin/bookings")
def get_all_bookings(user: models.User = Depends(get_current_user), db: Session = Depends(get_db)):
    if user.role not in ["admin", "staff", "executive"]:
        raise HTTPException(status_code=403, detail="Not authorized")
    
    # Eagerly load User and Computer relationships
    from sqlalchemy.orm import joinedload
    bookings = db.query(models.Booking).options(
        joinedload(models.Booking.user),
        joinedload(models.Booking.computer)
    ).all()
    
    return bookings

@app.put("/admin/users/{user_id}/info")
def update_user_info(user_id: int, request: UserAdminUpdate, user: models.User = Depends(get_current_user), db: Session = Depends(get_db)):
    if user.role not in ["admin", "executive"]:
        raise HTTPException(status_code=403, detail="Not authorized")
    
    db_user = db.query(models.User).filter(models.User.id == user_id).first()
    if not db_user:
        raise HTTPException(status_code=404, detail="User not found")
        
    if request.role is not None:
        db_user.role = request.role
    if request.department is not None:
        db_user.department = request.department

    db.commit()
    return {"message": "User info updated successfully"}

@app.get("/admin/usage-history")
def get_usage_history(user: models.User = Depends(get_current_user), db: Session = Depends(get_db)):
    if user.role not in ["admin", "executive", "staff"]:
        raise HTTPException(status_code=403, detail="Not authorized")
    
    from sqlalchemy.orm import joinedload
    
    # Get all bookings with user and computer info
    bookings = db.query(models.Booking).options(
        joinedload(models.Booking.user),
        joinedload(models.Booking.computer)
    ).all()
    
    # Get all usage logs with user and computer info 
    usage_logs = db.query(models.UsageLog).options(
        joinedload(models.UsageLog.user),
        joinedload(models.UsageLog.computer)
    ).all()
    
    return {
        "bookings": bookings,
        "usage_logs": usage_logs
    }