"""
NAME
  camera_shared.py

DESCRIPTION
  camera_shared.py contains the shared logic for left.py and right.py

AUTHOR
  Elliot Shine <elliot@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean). All Rights Reserved.

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
"""
import time
import os
import io
import threading
import datetime
from flask import jsonify, request, send_from_directory, Response
from picamera2 import Picamera2
from picamera2.encoders import MJPEGEncoder, H264Encoder
from picamera2.outputs import FileOutput
from libcamera import controls

# --- Configuration Constants ---
VIDEO_DIR = "recordings"
os.makedirs(VIDEO_DIR, exist_ok=True)

# --- Streaming Class ---
class StreamingOutput(io.BufferedIOBase):
    def __init__(self):
        self.frame = None
        self.condition = threading.Condition()
    def write(self, buf):
        with self.condition:
            self.frame = buf
            self.condition.notify_all()

# --- Core Camera Manager ---
class CameraManager:
    def __init__(self, sync_mode):
        self.picam2 = Picamera2()
        self.stream_output = StreamingOutput()
        self.recording_encoder = None
        self.current_res = (1920, 1080)
        self.preview_encoder = None
        self.ctrls = {'FrameRate': 24, 'SyncMode': sync_mode}

        # Initial Setup (Preview Mode)
        self.restart_preview()

    def restart_preview(self):
        """Stops everything and restarts in simple preview mode."""
        self.stop_camera()

        # Configure for Preview
        config = self.picam2.create_video_configuration(
            main={"size": self.current_res, "format": "YUV420"},
            lores={"size": (640, 480), "format": "YUV420"},
            controls=self.ctrls
        )
        self.picam2.configure(config)

        # Setup Preview Encoder (Jpeg with skip for performance)
        self.preview_encoder = MJPEGEncoder()
        
        self.picam2.start_encoder(self.preview_encoder, FileOutput(self.stream_output), name="lores")
        self.picam2.start()
        print("Camera: Preview Started")

    def stop_camera(self):
        """Safely stops recording and camera hardware."""
        if self.recording_encoder:
            self.picam2.stop_encoder(self.recording_encoder)
            self.recording_encoder = None

        # Stop preview encoder if running
        try: self.picam2.stop_encoder(self.preview_encoder)
        except: pass

        if self.picam2.started:
            self.picam2.stop()

    def prepare_and_start_recording(self, base_filename):
        """
        The CRITICAL sequence for sync:
        1. Stop Camera
        2. Configure (Fixed FPS, Sync Mode)
        3. Start Encoders (Arm them)
        4. Start Camera (Fire/Catch Pulse)
        """
        if self.recording_encoder:
            return False, "Already recording"

        self.stop_camera()

        config = self.picam2.create_video_configuration(
            main={"size": self.current_res, "format": "YUV420"},
            lores={"size": (640, 480), "format": "YUV420"},
            controls=self.ctrls
        )
        self.picam2.configure(config)

        # 3. Setup Encoders
        filepath = os.path.join(VIDEO_DIR, base_filename)

        # Main Recording Encoder (H.264 -> MP4 Container)
        rec_encoder = H264Encoder(bitrate=5000000)
        rec_encoder.sync_enable = True
        video_filename = os.path.join(VIDEO_DIR, f"{base_filename}.h264")
        pts_filename = os.path.join(VIDEO_DIR, f"{base_filename}.txt")

        output = FileOutput(video_filename, pts=pts_filename)
        
        self.preview_encoder = MJPEGEncoder()

        self.picam2.start_encoder(rec_encoder, output, name="main")
        self.picam2.start_encoder(self.preview_encoder, FileOutput(self.stream_output), name="lores")
        self.recording_encoder = rec_encoder

        self.picam2.start()

        return True, base_filename

    def capture_single_image(self, filename):
        """
        Performs a synchronized capture sequence:
        Stop -> Config(Sync) -> Start -> Capture -> Restart Preview
        """
        if self.recording_encoder:
            return False, "Cannot capture while recording"

        filepath = os.path.join(VIDEO_DIR, filename)

        try:
            # 1. Stop Preview
            self.stop_camera()

            config = self.picam2.create_video_configuration(
                main={"size": self.current_res, "format": "YUV420"},
                lores={"size": (640, 480), "format": "YUV420"},
                controls=self.ctrls
            )
            self.picam2.configure(config)

            # 3. Start Camera
            self.picam2.start()

            # 4. Capture Frame
            # We capture from the High-Res 'main' stream and convert to JPEG
            req = self.picam2.capture_sync_request()
            req.save("main", filepath)

            # 5. Restore Preview
            self.restart_preview()

            return True, filename

        except Exception as e:
            print(f"Photo Error: {e}")
            # Attempt to recover preview if capture failed
            try: self.restart_preview()
            except: pass
            return False, str(e)

    def apply_controls(self, data):
        """Applies controls from JSON data."""
        # Exposure / Gain / FPS
        if 'exposure' in data:
            self.ctrls['AeEnable'] = 0
            self.ctrls['ExposureTime'] = int(data['exposure'])
        if 'gain' in data:
            self.ctrls['AnalogueGain'] = float(data['gain'])
        if 'framerate' in data:
            self.ctrls['FrameRate'] = float(data['framerate'])
        if data.get('auto_exposure'):
            self.ctrls['AeEnable'] = 1

        # Focus Controls
        if 'af_mode' in data:
            self.ctrls['AfMode'] = int(data['af_mode'])
        if 'lens_position' in data:
            self.ctrls['LensPosition'] = float(data['lens_position'])
        if data.get('af_trigger'):
            self.ctrls['AfTrigger'] = 1

        try:
            self.picam2.set_controls(self.ctrls)
            return True, None
        except Exception as e:
            return False, str(e)

# --- Common Flask Routes Helper ---
def register_common_routes(app, camera_manager):
    """Registers the routes used by BOTH Left and Right nodes."""

    # Enable CORS for everything
    @app.after_request
    def after_request(response):
        response.headers.add('Access-Control-Allow-Origin', '*')
        response.headers.add('Access-Control-Allow-Headers', 'Content-Type,Authorization')
        response.headers.add('Access-Control-Allow-Methods', 'GET,PUT,POST,DELETE,OPTIONS')
        return response

    @app.route('/video_feed')
    def video_feed():
        def generate():
            while True:
                with camera_manager.stream_output.condition:
                    camera_manager.stream_output.condition.wait()
                    frame = camera_manager.stream_output.frame
                yield (b'--frame\r\n' b'Content-Type: image/jpeg\r\n\r\n' + frame + b'\r\n')
        return Response(generate(), mimetype='multipart/x-mixed-replace; boundary=frame')

    @app.route('/set_controls', methods=['POST'])
    def set_controls():
        success, err = camera_manager.apply_controls(request.json)
        return jsonify(success=success, message=err)

    @app.route('/set_resolution', methods=['POST'])
    def set_resolution():
        if camera_manager.recording_encoder:
            return jsonify(success=False, message="Recording in progress")

        w = int(request.json.get('width', 1920))
        h = int(request.json.get('height', 1080))
        camera_manager.current_res = (w, h)

        # Restart to apply resolution
        try:
            camera_manager.restart_preview()
            return jsonify(success=True)
        except Exception as e:
            return jsonify(success=False, message=str(e))

    @app.route('/stop_record', methods=['POST'])
    def stop_record():
        if camera_manager.recording_encoder:
            camera_manager.stop_camera()
            # Immediately restart preview so user sees feed
            camera_manager.restart_preview()
            return jsonify(success=True)
        return jsonify(success=False, message="Not recording")

    @app.route('/list_recordings')
    def list_recordings():
        try:
            files = sorted([f for f in os.listdir(VIDEO_DIR) if f.endswith(('.txt', '.h264', '.jpg'))], reverse=True)
            return jsonify(files)
        except: return jsonify([])

    @app.route('/recordings/<path:filename>')
    def download_file(filename):
        return send_from_directory(VIDEO_DIR, filename, as_attachment=True)

    @app.route('/delete_recording/<filename>', methods=['POST'])
    def delete_file(filename):
        try:
            os.remove(os.path.join(VIDEO_DIR, filename))
            return jsonify(success=True)
        except: return jsonify(success=False)
