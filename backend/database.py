from sqlalchemy import create_engine
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker

# รูปแบบ: mysql+pymysql://<user>:<password>@<host>/<db_name>
# ถ้ามีรหัสผ่าน ให้ใส่หลัง : เช่น root:1234@localhost/...
SQLALCHEMY_DATABASE_URL = "mysql+pymysql://root:@localhost/station_booking"

engine = create_engine(SQLALCHEMY_DATABASE_URL)

SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)

Base = declarative_base()