"""
NAME
  left.py

DESCRIPTION
  left.py is the executable that is run for the left camera in a stereoscopic
  configuration. It is responsible for serving a basic control website to the
  user and communicating with the right camera.

AUTHOR
  Elliot Shine <elliot@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean). All Rights Reserved.

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
"""
import requests
import datetime
from flask import Flask, jsonify, render_template, request
from libcamera import controls
import camera_shared # Import the shared file

# --- Configuration ---
RIGHT_IP = "192.168.1.115" # <--- Set IP of Right Node
app = Flask(__name__)
cam = camera_shared.CameraManager(controls.rpi.SyncModeEnum.Server)

# Register standard routes
camera_shared.register_common_routes(app, cam)

@app.route('/')
def index():
    # Assumes you have moved index.html to /templates folder
    return render_template('index.html', right_ip=RIGHT_IP)

@app.route('/set_controls_sync', methods=['POST'])
def set_controls_sync():
    # OVERRIDE: Left node must forward controls to Right node
    try:
        requests.post(f"http://{RIGHT_IP}:5000/set_controls", json=request.json, timeout=1)
    except: pass

    # Apply locally
    success, msg = cam.apply_controls(request.json)
    return jsonify(success=success, message=msg)

@app.route('/set_resolution_sync', methods=['POST'])
def set_resolution_sync():
    # OVERRIDE: Left node must forward controls to Right node
    try:
        requests.post(f"http://{RIGHT_IP}:5000/set_resolution", json=request.json, timeout=1)
    except: pass

    w = int(request.json.get('width', 1920))
    h = int(request.json.get('height', 1080))
    camera_manager.current_res = (w, h)

    # Restart to apply resolution
    try:
        camera_manager.restart_preview()
        return jsonify(success=True)
    except Exception as e:
        return jsonify(success=False, message=str(e))

@app.route('/start_sync_record', methods=['POST'])
def start_sync_record():
    if cam.recording_encoder:
        return jsonify(success=False, message="Already recording")

    try:
        # 1. Trigger Right Node (Client)
        # It will arm itself and wait for our pulse
        requests.post(f"http://{RIGHT_IP}:5000/start_record", timeout=5)

        # 3. Start Left Node (Server)
        # This will emit the pulses that start the Right node
        filename = datetime.datetime.now().strftime("left_%Y%m%d_%H%M%S")
        success, msg = cam.prepare_and_start_recording(filename)

        return jsonify(success=success, filename=filename, message=msg)
    except Exception as e:
        return jsonify(success=False, message=str(e))

@app.route('/stop_sync_record', methods=['POST'])
def stop_sync_record():
    # Stop locally
    if cam.recording_encoder:
        cam.stop_camera()
        cam.restart_preview()

    # Stop remote
    try: requests.post(f"http://{RIGHT_IP}:5000/stop_record", timeout=2)
    except: pass

    return jsonify(success=True)

@app.route('/capture_sync_photo', methods=['POST'])
def capture_sync_photo():
    if cam.recording_encoder:
        return jsonify(success=False, message="Recording in progress")

    try:
        # 1. Trigger Right Node (It will prepare and wait)
        try:
            requests.post(f"http://{RIGHT_IP}:5000/capture_photo", timeout=2)
        except Exception as e:
            return jsonify(success=False, message=f"Right node failed: {e}")

        # 2. Short delay to ensure Right Node is listening
        time.sleep(0.2)

        # 3. Capture Left (Server Mode - Sends Pulse)
        timestamp = datetime.datetime.now().strftime("%Y%m%d_%H%M%S")
        filename = f"left_photo_{timestamp}.jpg"

        success, result = cam.capture_single_image(filename)

        if success:
            return jsonify(success=True, filename=filename)
        else:
            return jsonify(success=False, message=result)

    except Exception as e:
        return jsonify(success=False, message=str(e))

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000, threaded=True)
