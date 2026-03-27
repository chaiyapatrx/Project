from sqlalchemy import Column, Integer, String, Boolean, DateTime, ForeignKey, Enum
from sqlalchemy.orm import relationship
from database import Base
import datetime
import enum

class UserRole(str, enum.Enum):
    ADMIN = "admin"
    STAFF = "staff"
    USER = "user"
    EXECUTIVE = "executive"

class User(Base):
    __tablename__ = "users"

    id = Column(Integer, primary_key=True, index=True)
    # MySQL ต้องระบุความยาว String สำหรับ Index/Unique เช่น String(50)
    username = Column(String(50), unique=True, index=True) 
    password_hash = Column(String(255)) 
    full_name = Column(String(100))
    role = Column(String(20)) 
    department = Column(String(100), nullable=True) 
    user_type = Column(String(50), nullable=True) 
    
    bookings = relationship("Booking", back_populates="user")
    logs = relationship("UsageLog", back_populates="user")

class Computer(Base):
    __tablename__ = "computers"

    id = Column(Integer, primary_key=True, index=True)
    name = Column(String(50), unique=True, index=True) # เช่น COM-01
    hwid = Column(String(100), nullable=True) # Hardware ID
    status = Column(String(20), default="available") 
    is_online = Column(Boolean, default=False)
    last_heartbeat = Column(DateTime, nullable=True)
    
    bookings = relationship("Booking", back_populates="computer")
    logs = relationship("UsageLog", back_populates="computer")
    commands = relationship("ComputerCommand", back_populates="computer")

class Booking(Base):
    __tablename__ = "bookings"

    id = Column(Integer, primary_key=True, index=True)
    user_id = Column(Integer, ForeignKey("users.id"))
    computer_id = Column(Integer, ForeignKey("computers.id"))
    start_time = Column(DateTime)
    end_time = Column(DateTime)
    access_code = Column(String(10)) 
    status = Column(String(20), default="pending") 

    user = relationship("User", back_populates="bookings")
    computer = relationship("Computer", back_populates="bookings")

class UsageLog(Base):
    __tablename__ = "usage_logs"

    id = Column(Integer, primary_key=True, index=True)
    user_id = Column(Integer, ForeignKey("users.id"))
    computer_id = Column(Integer, ForeignKey("computers.id"))
    login_time = Column(DateTime, default=datetime.datetime.utcnow)
    logout_time = Column(DateTime, nullable=True)
    
    user = relationship("User", back_populates="logs")
    computer = relationship("Computer", back_populates="logs")

class ComputerCommand(Base):
    __tablename__ = "computer_commands"

    id = Column(Integer, primary_key=True, index=True)
    computer_id = Column(Integer, ForeignKey("computers.id"))
    command_type = Column(String(50)) # 'shutdown', 'restart', 'logout'
    status = Column(String(20), default="pending") # 'pending', 'sent', 'executed'
    created_at = Column(DateTime, default=datetime.datetime.utcnow)
    
    computer = relationship("Computer", back_populates="commands")