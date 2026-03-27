from sqlalchemy import create_engine, text
from database import SQLALCHEMY_DATABASE_URL

def add_column():
    engine = create_engine(SQLALCHEMY_DATABASE_URL)
    with engine.begin() as conn:
        try:
            conn.execute(text("ALTER TABLE computers ADD COLUMN last_heartbeat DATETIME NULL;"))
            print("Column 'last_heartbeat' added successfully.")
        except Exception as e:
            print(f"Error (maybe column exists): {e}")

if __name__ == "__main__":
    add_column()
