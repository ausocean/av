"""
NAME
  right.py

DESCRIPTION
  right.py is the executable that is run for the right camera in a stereoscopic
  configuration. It listens for requests from the user (for the video stream)
  and the left camera.

AUTHOR
  Elliot Shine <elliot@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean). All Rights Reserved.

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
"""
from flask import Flask, jsonify
from libcamera import controls
import datetime
import threading
import camera_shared

app = Flask(__name__)
cam = camera_shared.CameraManager(controls.rpi.SyncModeEnum.Client)

# Register all the standard routes (controls, video feed, gallery)
camera_shared.register_common_routes(app, cam)

@app.route('/start_record', methods=['POST'])
def start_record():
    # RIGHT NODE: Starts as CLIENT (Passive)
    base_filename = datetime.datetime.now().strftime("right_%Y%m%d_%H%M%S")

    # Use a thread to avoid blocking the Left node's request (Deadlock prevention)
    def _run_recording():
        print(f"THREAD: Starting recording task for {base_filename}...")
        try:
            cam.prepare_and_start_recording(base_filename)
            print(f"THREAD: Recording task finished for {base_filename}")
        except Exception as e:
            print(f"THREAD ERROR: {e}")

    threading.Thread(target=_run_recording).start()

    return jsonify(success=True, filename=base_filename)

@app.route('/capture_photo', methods=['POST'])
def capture_photo():
    # Define the background task
    def _run_capture():
        timestamp = datetime.datetime.now().strftime("%Y%m%d_%H%M%S")
        filename = f"right_photo_{timestamp}.jpg"
        # Start as Client (Passive)
        cam.capture_single_image(filename)

    # Start thread immediately
    t = threading.Thread(target=_run_capture)
    t.start()

    return jsonify(success=True)


if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000, threaded=True)
