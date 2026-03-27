from sqlalchemy import create_engine, inspect, text
from database import SQLALCHEMY_DATABASE_URL, SessionLocal
import models

def check_system():
    print("=== System Diagnostic ===")
    
    # 1. Check Tables
    engine = create_engine(SQLALCHEMY_DATABASE_URL)
    inspector = inspect(engine)
    tables = inspector.get_table_names()
    print(f"Tables found: {tables}")
    
    if "computer_commands" not in tables:
        print("CRITICAL: 'computer_commands' table MISSING!")
    else:
        print("OK: 'computer_commands' table exists.")

    db = SessionLocal()
    
    # 2. Check Computers
    computers = db.query(models.Computer).all()
    print(f"\nComputers found: {len(computers)}")
    for c in computers:
        print(f" - [{c.id}] {c.name} (HWID: {c.hwid}) | Online: {c.is_online} | Last Heartbeat: {c.last_heartbeat}")

    # 3. Check Commands
    try:
        commands = db.query(models.ComputerCommand).all()
        print(f"\nTotal Commands: {len(commands)}")
        for cmd in commands:
            print(f" - ID: {cmd.id} | ComputerID: {cmd.computer_id} | Type: {cmd.command_type} | Status: {cmd.status}")
    except Exception as e:
        print(f"Error reading commands: {e}")

    db.close()

if __name__ == "__main__":
    check_system()
