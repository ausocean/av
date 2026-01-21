import time
import io
import os
import datetime
import threading
import requests
from flask import Flask, Response, render_template_string, jsonify, send_from_directory, request
from libcamera import controls
from picamera2 import Picamera2
from picamera2.encoders import MJPEGEncoder, H264Encoder
from picamera2.outputs import FileOutput

# --- Configuration ---
RIGHT_IP = "192.168.8.208"  # <--- CHANGE THIS
VIDEO_DIR = "recordings"
os.makedirs(VIDEO_DIR, exist_ok=True)
CURRENT_RES = (1920, 1080)

app = Flask(__name__)

# --- Camera Setup (Server Mode) ---
picam2 = Picamera2()
picam2.options["quality"] = 95
ctrls = {'SyncMode': controls.rpi.SyncModeEnum.Server, 'SyncFrames': 100, 'FrameRate': 24}
config = picam2.create_video_configuration(
    main={"size": CURRENT_RES, "format": "YUV420"},
    lores={"size": (640, 480), "format": "YUV420"},
    controls=ctrls
)
picam2.configure(config)
picam2.start()

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

# --- HTML Interface (Restored & Complete) ---
HTML_PAGE = f"""
<!DOCTYPE html>
<html>
<head>
    <title>Dual Pi Sync</title>
    <style>
        body {{ font-family: sans-serif; text-align: center; background: #222; color: #ddd; margin:0; padding: 20px; }}
        
        .cam-container {{ display: flex; justify-content: center; gap: 20px; flex-wrap: wrap; }}
        .cam-box {{ width: 45%; min-width: 400px; background: #333; padding: 10px; border-radius: 8px; }}
        img {{ width: 100%; border: 2px solid #555; border-radius: 4px; }}
        
        /* Settings Box */
        .settings-container {{ display: flex; justify-content: center; gap: 20px; margin-top: 20px; flex-wrap: wrap; }}
        .settings-box {{ background: #333; padding: 15px; border-radius: 8px; text-align: left; min-width: 320px; }}
        .settings-box h3 {{ margin-top: 0; border-bottom: 1px solid #555; padding-bottom: 5px; }}
        .settings-row {{ margin-bottom: 12px; display: flex; align-items: center; justify-content: space-between; }}
        .settings-row label {{ width: 100px; color: #aaa; }}
        input[type=range] {{ flex-grow: 1; margin: 0 10px; cursor: pointer; }}
        select, button {{ padding: 6px; border-radius: 4px; border: none; cursor: pointer; }}
        
        /* Main Controls */
        .controls {{ margin: 30px 0; padding: 20px; background: #2a2a2a; border-radius: 10px; display: inline-block; }}
        .main-btn {{ padding: 15px 30px; font-size: 18px; border-radius: 8px; border: none; cursor: pointer; margin: 0 10px; font-weight: bold; }}
        #btn-photo {{ background: #007bff; color: white; }}
        #btn-start {{ background: #28a745; color: white; }}
        #btn-stop {{ background: #dc3545; color: white; display: none; }}
        
        /* Gallery */
        .gallery-section {{ display: flex; gap: 20px; justify-content: center; margin-top: 40px; text-align: left; }}
        .gallery-col {{ width: 45%; background: #2a2a2a; padding: 15px; border-radius: 8px; }}
        .file-item {{ display: flex; justify-content: space-between; padding: 8px; border-bottom: 1px solid #444; }}
        .btn-dl {{ background: #444; color: white; padding: 3px 8px; text-decoration:none; border-radius: 4px; font-size: 0.8em; }}
    </style>
</head>
<body onload="loadAllGalleries(); initControls();">
    <h1>Synchronized Capture System</h1>
    
    <div class="cam-container">
        <div class="cam-box">
            <h2>Left</h2>
            <img src="/video_feed" />
        </div>
        <div class="cam-box">
            <h2>Right</h2>
            <img src="http://{RIGHT_IP}:5000/video_feed" />
        </div>
    </div>

    <div class="settings-container">
        <div class="settings-box">
            <h3>Image Settings</h3>
            <div class="settings-row">
                <label>Resolution:</label>
                <select id="res-select" onchange="changeResolution()">
                    <option value="1280,720">720p</option>
                    <option value="1920,1080" selected>1080p</option>
                    <option value="2304,1296">1296p (3MP)</option>
                </select>
            </div>
            <div class="settings-row">
                <label>Exposure:</label>
                <input type="range" id="exp-slider" min="100" max="30000" step="100" value="10000" oninput="updateControls()">
                <span id="exp-val" style="width: 50px; text-align: right;">Auto</span>
            </div>
             <div class="settings-row" style="justify-content: flex-end;">
                <button onclick="setAutoExposure()" style="background: #444; color: white;">Set Auto Exp</button>
            </div>
             <div class="settings-row">
                <label>Framerate:</label>
                <input type="number" id="fps-input" value="24" min="1" max="60" style="width: 50px;" onchange="updateControls()">
            </div>
        </div>

        <div class="settings-box">
            <h3>Focus Control</h3>
            <div class="settings-row">
                <label>Mode:</label>
                <select id="af-mode" onchange="changeFocusMode()">
                    <option value="2" selected>Continuous (CAF)</option>
                    <option value="1">Auto (One-Shot)</option>
                    <option value="0">Manual</option>
                </select>
            </div>
            
            <div class="settings-row" id="manual-focus-row" style="opacity: 0.5; pointer-events: none;">
                <label>Position:</label>
                <input type="range" id="focus-slider" min="0" max="15" step="0.1" value="0" oninput="updateFocus()">
                <span id="focus-val" style="width: 30px;">Inf</span>
            </div>

            <div class="settings-row" id="auto-focus-row" style="display: none;">
                <button onclick="triggerAutofocus()" style="width: 100%; background: #007bff; color: white; padding: 10px;">Trigger Autofocus Now</button>
            </div>
        </div>
    </div>

    <div class="controls">
        <button id="btn-photo" class="main-btn" onclick="takePhoto()">Take Photo</button>
        <button id="btn-start" class="main-btn" onclick="toggleRecord(true)">Start Video</button>
        <button id="btn-stop" class="main-btn" onclick="toggleRecord(false)">Stop Video</button>
        <div id="status" style="margin-top: 15px; color: #aaa;">Ready</div>
    </div>

    <div class="gallery-section">
        <div class="gallery-col">
            <h2>Left Files</h2>
            <div id="gallery-left">Loading...</div>
        </div>
        <div class="gallery-col">
            <h2>Right Files</h2>
            <div id="gallery-right">Loading...</div>
        </div>
    </div>

    <script>
        const RIGHT_API = "http://{RIGHT_IP}:5000";

        function initControls() {{
            // Set initial UI state
            changeFocusMode();
        }}

        // --- Focus Logic ---
        function changeFocusMode() {{
            const mode = parseInt(document.getElementById('af-mode').value);
            const manualRow = document.getElementById('manual-focus-row');
            const autoRow = document.getElementById('auto-focus-row');
            
            if (mode === 0) {{ // Manual
                manualRow.style.opacity = "1";
                manualRow.style.pointerEvents = "auto";
                autoRow.style.display = "none";
            }} else if (mode === 1) {{ // Auto One-Shot
                manualRow.style.opacity = "0.5";
                manualRow.style.pointerEvents = "none";
                autoRow.style.display = "flex";
            }} else {{ // Continuous
                manualRow.style.opacity = "0.5";
                manualRow.style.pointerEvents = "none";
                autoRow.style.display = "none";
            }}

            // Send command
            fetch('/set_controls', {{
                method: 'POST',
                headers: {{'Content-Type': 'application/json'}},
                body: JSON.stringify({{ af_mode: mode }})
            }});
        }}

        function updateFocus() {{
            const val = document.getElementById('focus-slider').value;
            document.getElementById('focus-val').innerText = val;
            fetch('/set_controls', {{
                method: 'POST',
                headers: {{'Content-Type': 'application/json'}},
                body: JSON.stringify({{ af_mode: 0, lens_position: val }})
            }});
        }}

        function triggerAutofocus() {{
            fetch('/set_controls', {{
                method: 'POST',
                headers: {{'Content-Type': 'application/json'}},
                body: JSON.stringify({{ af_mode: 1, af_trigger: true }})
            }});
        }}

        // --- Exposure & Image Logic ---
        function updateControls() {{
            const exposure = document.getElementById('exp-slider').value;
            const fps = document.getElementById('fps-input').value;
            
            document.getElementById('exp-val').innerText = exposure;
            
            fetch('/set_controls', {{
                method: 'POST',
                headers: {{'Content-Type': 'application/json'}},
                body: JSON.stringify({{ exposure: exposure, framerate: fps }})
            }});
        }}

        function setAutoExposure() {{
            document.getElementById('exp-val').innerText = "Auto";
            fetch('/set_controls', {{
                method: 'POST',
                headers: {{'Content-Type': 'application/json'}},
                body: JSON.stringify({{ auto_exposure: true }})
            }});
        }}

        function changeResolution() {{
            const val = document.getElementById('res-select').value.split(',');
            if(!confirm("Cameras will briefly restart. Continue?")) return;
            
            fetch('/set_resolution', {{
                method: 'POST',
                headers: {{'Content-Type': 'application/json'}},
                body: JSON.stringify({{ width: parseInt(val[0]), height: parseInt(val[1]) }})
            }}).then(() => setTimeout(() => location.reload(), 3000));
        }}

        // --- Capture Logic ---
        function takePhoto() {{
            document.getElementById('status').innerText = "Syncing & Capturing...";
            fetch('/capture_sync_photo', {{ method: 'POST' }})
                .then(res => res.json())
                .then(data => {{
                    if(data.success) {{
                        document.getElementById('status').innerText = "Photo Saved: " + data.filename;
                        setTimeout(loadAllGalleries, 1000);
                    }} else alert(data.message);
                }});
        }}

        function toggleRecord(start) {{
            const endpoint = start ? '/start_sync_record' : '/stop_sync_record';
            document.getElementById('status').innerText = "Syncing...";
            fetch(endpoint, {{ method: 'POST' }})
                .then(res => res.json())
                .then(data => {{
                    if(data.success) {{
                        document.getElementById('btn-start').style.display = start ? 'none' : 'inline-block';
                        document.getElementById('btn-stop').style.display = start ? 'inline-block' : 'none';
                        document.getElementById('status').innerText = start ? "Recording..." : "Stopped.";
                        if(!start) setTimeout(loadAllGalleries, 1000);
                    }} else alert(data.message);
                }});
        }}

        // --- Gallery ---
        function loadAllGalleries() {{
            loadGallery('/list_recordings', 'gallery-left', '');
            loadGallery(RIGHT_API + '/list_recordings', 'gallery-right', RIGHT_API);
        }}

        function loadGallery(url, elementId, baseUrl) {{
            fetch(url).then(res => res.json()).then(files => {{
                const container = document.getElementById(elementId);
                container.innerHTML = '';
                if(files.length === 0) container.innerHTML = '<div style="color:#666">Empty</div>';
                
                files.forEach(f => {{
                    const dl = baseUrl ? baseUrl + '/recordings/' + f : '/recordings/' + f;
                    const d = document.createElement('div');
                    d.className = 'file-item';
                    d.innerHTML = `<span>${{f}}</span><a href="${{dl}}" class="btn-dl" download>Download</a>`;
                    container.appendChild(d);
                }});
            }}).catch(() => document.getElementById(elementId).innerText = "Offline");
        }}
    </script>
</body>
</html>
"""

# --- Routes ---

@app.route('/')
def index():
    return render_template_string(HTML_PAGE)

@app.route('/set_controls', methods=['POST'])
def set_controls():
    data = request.json
    ctrls = {}
    
    # Map controls
    if 'exposure' in data:
        ctrls['AeEnable'] = 0
        ctrls['ExposureTime'] = int(data['exposure'])
    if 'framerate' in data:
        ctrls['FrameRate'] = float(data['framerate'])
    if data.get('auto_exposure'):
        ctrls['AeEnable'] = 1
    if 'af_mode' in data:
        ctrls['AfMode'] = int(data['af_mode'])
    if 'lens_position' in data:
        ctrls['LensPosition'] = float(data['lens_position'])
    if data.get('af_trigger'):
        ctrls['AfTrigger'] = 1

    try:
        # 1. Apply Local
        picam2.set_controls(ctrls)
        # 2. Forward to Right
        try: requests.post(f"http://{RIGHT_IP}:5000/set_controls", json=data, timeout=1)
        except: pass
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
        # Forward to Right First
        try: requests.post(f"http://{RIGHT_IP}:5000/set_resolution", json=request.json, timeout=5)
        except: pass

        # Reconfigure Left
        picam2.stop_recording()
        picam2.stop()
        ctrls = {'SyncMode': controls.rpi.SyncModeEnum.Server}
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

@app.route('/capture_sync_photo', methods=['POST'])
def capture_sync_photo():
    if recording_encoder: return jsonify(success=False, message="Recording in progress")
    try:
        # 1. Trigger Right
        requests.post(f"http://{RIGHT_IP}:5000/capture_photo", timeout=2)
        
        # 2. Resync Left
        picam2.stop_recording()
        picam2.stop()
        ctrls = {'SyncMode': controls.rpi.SyncModeEnum.Server}
        config = picam2.create_video_configuration(
            main={"size": CURRENT_RES, "format": "YUV420"},
            lores={"size": (640, 480), "format": "YUV420"},
            controls=ctrls
        )
        picam2.configure(config)
        picam2.start() # Trigger pulse
        
        # 3. Capture
        filename = datetime.datetime.now().strftime("photo_left_%Y%m%d_%H%M%S.jpg")
        filepath = os.path.join(VIDEO_DIR, filename)
        picam2.capture_file(filepath, format="jpeg")
        
        # 4. Restore
        picam2.start_recording(MJPEGEncoder(), FileOutput(stream_output), name="lores")
        return jsonify(success=True, filename=filename)
    except Exception as e:
        return jsonify(success=False, message=str(e))

@app.route('/start_sync_record', methods=['POST'])
def start_sync_record():
    global recording_encoder
    
    # 1. Capture current settings BEFORE stopping
    current_controls = picam2.controls.make_dict()
    
    if recording_encoder: return jsonify(success=False)
    try:
        # Resync Left
        picam2.stop_recording()
        picam2.stop()

        requests.post(f"http://{RIGHT_IP}:5000/start_record", timeout=10)
        
        # 2. Force SyncMode.Server, but keep all other settings
        current_controls['SyncMode'] = controls.rpi.SyncModeEnum.Server
        
        # 3. Apply saved controls to new configuration
        config = picam2.create_video_configuration(
            main={"size": CURRENT_RES, "format": "YUV420"},
            lores={"size": (640, 480), "format": "YUV420"},
            controls=current_controls
        )
        encoder = H264Encoder(bitrate=5000000)
        encoder.sync_enable = True
        mjpeg_encoder = MJPEGEncoder()
        picam2.configure(config)

        base_name = datetime.datetime.now().strftime("left_%Y%m%d_%H%M%S")
        video_filename = os.path.join(VIDEO_DIR, f"{base_name}.h264")
        pts_filename = os.path.join(VIDEO_DIR, f"{base_name}.txt")
    
        output = FileOutput(video_filename, pts=pts_filename)

        picam2.start_encoder(encoder, output, name="main")
        picam2.start_encoder(mjpeg_encoder, FileOutput(stream_output), name="lores")
        picam2.start()
        encoder.sync.wait()

        recording_encoder = encoder
        return jsonify(success=True)
    except Exception as e:
        return jsonify(success=False, message=str(e))

@app.route('/stop_sync_record', methods=['POST'])
def stop_sync_record():
    global recording_encoder
    if recording_encoder:
        picam2.stop_encoder(recording_encoder)
        recording_encoder = None
    try: requests.post(f"http://{RIGHT_IP}:5000/stop_record", timeout=2)
    except: pass
    return jsonify(success=True)

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
        files = sorted([f for f in os.listdir(VIDEO_DIR) if f.endswith(('.h264', '.mp4', '.jpg', '.txt'))], reverse=True)
        return jsonify(files)
    except: return jsonify([])

@app.route('/recordings/<path:filename>')
def download_file(filename):
    return send_from_directory(VIDEO_DIR, filename, as_attachment=True)

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000, threaded=True)
