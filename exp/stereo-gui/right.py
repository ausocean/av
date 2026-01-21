import time
import io
import os
import datetime
import threading
from flask import Flask, Response, jsonify, send_from_directory, request
from libcamera import controls
from picamera2 import Picamera2
from picamera2.encoders import MJPEGEncoder, H264Encoder
from picamera2.outputs import FileOutput

# --- Configuration ---
VIDEO_DIR = "recordings"
os.makedirs(VIDEO_DIR, exist_ok=True)
CURRENT_RES = (1920, 1080) # Default resolution

app = Flask(__name__)

# --- CORS Helper ---
@app.after_request
def after_request(response):
    response.headers.add('Access-Control-Allow-Origin', '*')
    response.headers.add('Access-Control-Allow-Headers', 'Content-Type,Authorization')
    response.headers.add('Access-Control-Allow-Methods', 'GET,PUT,POST,DELETE,OPTIONS')
    return response

# --- Camera Setup (Client Mode) ---
picam2 = Picamera2()
ctrls = {'SyncMode': controls.rpi.SyncModeEnum.Client, 'FrameRate': 24}
config = picam2.create_video_configuration(
    main={"size": CURRENT_RES},
    lores={"size": (640, 480)},
    controls=ctrls
)
picam2.configure(config)
picam2.start()

# --- Streaming Setup ---
class StreamingOutput(io.BufferedIOBase):
    def __init__(self):
        self.frame = None
        self.condition = threading.Condition()
    def write(self, buf):
        with self.condition:
            self.frame = buf
            self.condition.notify_all()

stream_output = StreamingOutput()
picam2.start_recording(MJPEGEncoder(), FileOutput(stream_output), name="lores")
recording_encoder = None

# --- Background Capture Sequence ---
def run_sync_capture_sequence():
    """Stops, re-syncs in single-frame mode, captures, and restarts."""
    filename = datetime.datetime.now().strftime("photo_right_%Y%m%d_%H%M%S.jpg")
    filepath = os.path.join(VIDEO_DIR, filename)
    try:
        picam2.stop_recording()
        picam2.stop()
        
        # Client Mode for Sync
        ctrls = {'SyncMode': controls.rpi.SyncModeEnum.Client}
        config = picam2.create_video_configuration(
            main={"size": CURRENT_RES, "format": "YUV420"},
            lores={"size": (640, 480), "format": "YUV420"},
            controls=ctrls
        )
        picam2.configure(config)
        picam2.start()
        
        # Capture 1 JPEG
        picam2.capture_file(filepath, format="jpeg")
        
        # Restart Stream
        picam2.start_recording(MJPEGEncoder(), FileOutput(stream_output), name="lores")
    except Exception as e:
        print(f"Photo Error: {e}")

# --- Routes ---

@app.route('/set_controls', methods=['POST'])
def set_controls():
    data = request.json
    ctrls = {}
    
    # Exposure / Gain / FPS
    if 'exposure' in data:
        ctrls['AeEnable'] = 0 
        ctrls['ExposureTime'] = int(data['exposure'])
    if 'gain' in data:
        ctrls['AnalogueGain'] = float(data['gain'])
    if 'framerate' in data:
        ctrls['FrameRate'] = float(data['framerate'])
    if data.get('auto_exposure'):
        ctrls['AeEnable'] = 1
        
    # Focus Controls
    if 'af_mode' in data:
        ctrls['AfMode'] = int(data['af_mode'])
    if 'lens_position' in data:
        ctrls['LensPosition'] = float(data['lens_position'])
    if data.get('af_trigger'):
        ctrls['AfTrigger'] = 1

    try:
        picam2.set_controls(ctrls)
        return jsonify(success=True)
    except Exception as e:
        return jsonify(success=False, message=str(e))

@app.route('/set_resolution', methods=['POST'])
def set_resolution():
    global recording_encoder, CURRENT_RES
    if recording_encoder: return jsonify(success=False, message="Recording in progress")
    
    w, h = int(request.json.get('width', 1920)), int(request.json.get('height', 1080))
    CURRENT_RES = (w, h)
    
    try:
        picam2.stop_recording()
        picam2.stop()
        ctrls = {'SyncMode': controls.rpi.SyncModeEnum.Client}
        config = picam2.create_video_configuration(
            main={"size": CURRENT_RES, "format": "YUV420"},
            lores={"size": (640, 480), "format": "YUV420"},
            controls=ctrls
        )
        picam2.configure(config)
        picam2.start()
        picam2.start_recording(MJPEGEncoder(), FileOutput(stream_output), name="lores")
        return jsonify(success=True)
    except Exception as e:
        return jsonify(success=False, message=str(e))

@app.route('/capture_photo', methods=['POST'])
def capture_photo():
    if recording_encoder: return jsonify(success=False, message="Recording in progress")
    t = threading.Thread(target=run_sync_capture_sequence)
    t.start()
    return jsonify(success=True)

@app.route('/start_record', methods=['POST'])
def start_record():
    global recording_encoder
    # 1. Capture the current settings (Exposure, Focus, etc.) BEFORE stopping
    current_controls = picam2.controls.make_dict()
    
    if recording_encoder: return jsonify(success=False)

    try:
        picam2.stop_recording()
        picam2.stop()
        
        # 2. Merge SyncMode into the saved controls
        current_controls['SyncMode'] = controls.rpi.SyncModeEnum.Client
        
        # 3. Pass 'controls=current_controls' to the configuration
        config = picam2.create_video_configuration(
            main={"size": CURRENT_RES, "format": "YUV420"},
            lores={"size": (640, 480), "format": "YUV420"},
            controls=current_controls
        )
        picam2.configure(config)
        encoder = H264Encoder(bitrate=5000000)
        encoder.sync_enable = True 
        mjpeg_encoder = MJPEGEncoder()

        base_name = datetime.datetime.now().strftime("right_%Y%m%d_%H%M%S")
        video_filename = os.path.join(VIDEO_DIR, f"{base_name}.h264")
        pts_filename = os.path.join(VIDEO_DIR, f"{base_name}.txt")
    
        output = FileOutput(video_filename, pts=pts_filename)
        picam2.start_encoder(encoder, output, name="main")
        picam2.start_encoder(mjpeg_encoder, FileOutput(stream_output), name="lores")
        picam2.start()

        recording_encoder = encoder
        return jsonify(success=True, filename=filename)
    except Exception as e:
        return jsonify(success=False, message=str(e))

@app.route('/stop_record', methods=['POST'])
def stop_record():
    global recording_encoder
    if recording_encoder:
        picam2.stop_encoder(recording_encoder)
        recording_encoder = None
        return jsonify(success=True)
    return jsonify(success=False)

@app.route('/video_feed')
def video_feed():
    def generate():
        while True:
            with stream_output.condition:
                stream_output.condition.wait()
                frame = stream_output.frame
            yield (b'--frame\r\n' b'Content-Type: image/jpeg\r\n\r\n' + frame + b'\r\n')
    return Response(generate(), mimetype='multipart/x-mixed-replace; boundary=frame')

@app.route('/list_recordings')
def list_recordings():
    try:
        # Return both videos and photos
        files = sorted([f for f in os.listdir(VIDEO_DIR) if f.endswith(('.h264', '.mp4', '.jpg', '.txt'))], reverse=True)
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

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000, threaded=True)
