# Stereoscopic Recording GUI
This is a GUI application that allows for easy recording of stereoscopic video using two underwater cameras.
## Installation
Install all dependencies using `sudo apt install python3-flask python3-requests python3-camera2` on both cameras.
## Usage
1. Stop the ausocean service on both cameras (to ensure that no rv processes are running that use the camera) `sudo systemctl stop ausocean.service`
2. Find the right camera's ip address using `ip a` and change the `RIGHT_IP` variable at the top of left.py accordingly
3. Start the right camera using `LIBCAMERA_LOG_LEVELS="RPiSync:DEBUG" python3 right.py`
4. Start the left camera using `python3 left.py`
5. Open http://{LEFT_CAMERA_IP}:5000 in your browser - this is also given in the output of the server command

To Record Stereoscopic Video:
1. Set the exposure to an appropriate value (do not use auto exposure), set the focus mode to manual and set the focus to an appropriate value
2. Press start video
3. Check the output of the right camera - you should see a line similar to `INFO RPiSync sync.cpp:281 *** Sync achieved! Difference 25us`. The difference should be less than 150us. If it isn't, then stop the recording and start it again until it is less than 150us. The corrections should be less than 100us after sync is achieved, preferably mostly zero.
4. When you wish to stop recording, press stop video.

To Play Stereoscopic Video Recordings:
1. Download the .h264 and .txt files from both the left and the right cameras using the file list at the bottom of the page.
2. Add `# timecode format v2` to the top of each txt file - for example you can use `sed -i '1i# timecode format v2' {file}.txt`
3. Use mkvmerge from mkvtoolnix to convert to a mkv for playback `mkvmerge -o {file}.mkv --timecodes 0:{file}.txt {file}.h264`
