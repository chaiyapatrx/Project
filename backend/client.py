# client.py
import time
import requests
import atexit
import socket

# ตั้งค่าพื้นฐาน
SERVER_URL = "http://127.0.0.1:8000"
MACHINE_NAME = "COM-01"  # สมมติว่าเครื่องนี้ชื่อ COM-01

def send_status(online=True, occupied=False, user=None):
    """ฟังก์ชันส่งสถานะไปยัง Server"""
    try:
        payload = {
            "name": MACHINE_NAME,
            "is_online": online,
            "is_occupied": occupied,
            "current_user": user
        }
        response = requests.post(f"{SERVER_URL}/update-status", json=payload)
        if response.status_code == 200:
            print(f"✅ ส่งสถานะสำเร็จ: Online={online}, User={user}")
        else:
            print(f"⚠️ Server ตอบกลับผิดปกติ: {response.status_code}")
    except Exception as e:
        print(f"❌ เชื่อมต่อ Server ไม่ได้: {e}")

def on_exit():
    """ฟังก์ชันที่จะทำงานเมื่อโปรแกรมถูกปิด (เช่น ปิดเครื่อง)"""
    print("กำลังปิดโปรแกรม... แจ้ง Server ว่า Offline")
    send_status(online=False, occupied=False, user=None)

# ลงทะเบียนให้ทำ on_exit ก่อนปิดโปรแกรม
atexit.register(on_exit)

# --- ส่วนทำงานหลัก ---
if __name__ == "__main__":
    print(f"🖥️  Client เริ่มทำงานที่เครื่อง: {MACHINE_NAME}")
    
    # 1. แจ้งว่าออนไลน์ทันทีที่เปิดโปรแกรม
    send_status(online=True)

    # 2. วนลูปส่ง Heartbeat ทุกๆ 5 วินาที
    try:
        while True:
            # สมมติสถานะ: (อนาคตตรงนี้เราจะเขียนโค้ดเช็คว่าใคร Login อยู่จริงไหม)
            current_user = None 
            is_occupied = False
            
            send_status(online=True, occupied=is_occupied, user=current_user)
            
            # รอ 5 วินาทีก่อนส่งครั้งต่อไป
            time.sleep(5)
            
    except KeyboardInterrupt:
        # กด Ctrl+C เพื่อหยุด
        print("\nหยุดการทำงาน")