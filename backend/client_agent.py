import requests
import time
import socket
import sys
import uuid
import atexit
import signal
import json
import os

# Configuration File
CONFIG_FILE = "agent_config.json"
INTERVAL = 5 # seconds

def load_or_ask_config():
    if os.path.exists(CONFIG_FILE):
        try:
            with open(CONFIG_FILE, "r") as f:
                config = json.load(f)
                return config.get("server_ip", "localhost")
        except:
            pass
    
    print("--- First Time Setup ---")
    server_ip = input("Enter Server IP Address (e.g., 192.168.1.50 or localhost): ").strip()
    if not server_ip:
        server_ip = "localhost"
    
    with open(CONFIG_FILE, "w") as f:
        json.dump({"server_ip": server_ip}, f)
    
    print(f"Configuration saved to {CONFIG_FILE}")
    return server_ip

# Load Config
SERVER_IP = load_or_ask_config()
API_URL = f"http://{SERVER_IP}:8000/api/heartbeat"

def get_hwid():
    # Use getnode for MAC address based ID (stable for physical machines)
    return str(uuid.getnode())

def get_computer_name():
    return socket.gethostname()

def send_heartbeat(name, hwid, online=True):
    payload = {
        "computer_name": name,
        "hwid": hwid,
        "is_online": online
    }
    
    try:
        response = requests.post(API_URL, json=payload, timeout=2)
        status_msg = "Online" if online else "Offline"
        if response.status_code == 200:
            print(f"[{time.strftime('%H:%M:%S')}] Heartbeat ({status_msg}) sent to {SERVER_IP} for {name}: OK")
        else:
            print(f"[{time.strftime('%H:%M:%S')}] Failed: {response.status_code} - {response.text}")
    except requests.exceptions.ConnectionError:
        print(f"[{time.strftime('%H:%M:%S')}] Connection Error: Could not connect to {API_URL}")
        print(f"   -> Check if Server is running at {SERVER_IP}")
        print(f"   -> Check if Firewalls allow port 8000")
    except Exception as e:
        print(f"[{time.strftime('%H:%M:%S')}] Error: {str(e)}")

def on_exit():
    print("\n[System] Shutting down agent...")
    name = get_computer_name()
    hwid = get_hwid()
    send_heartbeat(name, hwid, online=False)
    print("[System] Offline signal sent. Goodbye.")

if __name__ == "__main__":
    atexit.register(on_exit)
    signal.signal(signal.SIGTERM, lambda num, frame: sys.exit(0))
    signal.signal(signal.SIGINT, lambda num, frame: sys.exit(0))

    computer_name = get_computer_name()
    hwid = get_hwid()
    
    print(f"\n--- Client Agent Started ---")
    print(f"Computer: {computer_name}")
    print(f"HWID: {hwid}")
    print(f"Target Server: {API_URL}")
    print("Press Ctrl+C to stop.\n")

    while True:
        send_heartbeat(computer_name, hwid, online=True)
        time.sleep(INTERVAL)
