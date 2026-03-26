# Stereoscopic Recording GUI
This is a GUI application that allows for easy recording of stereoscopic video using two underwater cameras.
## Installation
Install all dependencies using `sudo apt install python3-flask python3-requests python3-camera2 mkvtoolnix` on both cameras.
## Usage
1. Stop the ausocean service on both cameras (to ensure that no rv processes are running that use the camera) `sudo systemctl stop ausocean.service`
2. Find the right camera's ip address using `ip a` and change the `RIGHT_IP` variable at the top of left.py accordingly
3. Start the right camera using `python3 right.py`
4. Start the left camera using `python3 left.py`
5. Open http://{LEFT_CAMERA_IP}:5000 in your browser - this is also given in the output of the left camera

To Record Stereoscopic Video:
1. Set the exposure to an appropriate value (DO NOT use auto exposure), set the focus mode to manual and set the focus to an appropriate value
2. Press start video
3. Wait for the sync to be achieved. If the sync difference is greater than 150us or there are corrections more than once every 10 seconds, stop the recording and start it again.
4. When you wish to stop recording, press stop video.

To Play Stereoscopic Video Recordings:
Wait a few seconds after the recording, refresh the page, and download the MKV files. There is also a csv containing the initial sync difference and any corrections.
