# Stereo Sync Analyser

A toolset for analyzing the synchronization precision of stereoscopic video recordings by detecting visual clocks in the video frames.

## Included Tools

### 1. `sync_analyser.py`
An interactive tool that allows you to view the synchronization offset frame-by-frame.

- **Usage**: `python3 sync_analyser.py path/to/left_video.mkv path/to/right_video.mkv`
- **Controls**:
  - `Space` or `P`: Pause/Play
  - `N`: Next frame (when paused)
  - `Q`: Quit and show final results

### 2. `bulk_sync_analyser.py`
A script that processes entire folders of video recordings and exports the results to a CSV.

- **Usage**: `python3 bulk_sync_analyser.py path/to/video_folder/`
- **Output**: Generates `sync_results.csv` with a summary of the mean offset and jitter (standard deviation) for every video pair found.

### 3. `sync_test.html`
A simple webpage that displays a sync clock, a timestamp with thousandths of a second, and a frame counter running at 24 fps. This should be used on a monitor with a high refresh rate (120Hz+).

- **Usage**: Open `sync_test.html` in your browser.

---

## How it Works

The analyser works by detecting two visual markers in each frame:
1. **The Clock**: A circular region identified by green corner markers. It calculates the angle of a rotating "second hand" (white dot vs. red anchor).
2. **The Info Box**: A rectangular region identified by 4 blue corner markers. This is warped to a flat view for secondary data (like timestamps).

### Sync Calculation
By comparing the angle of the clock hand between the left and right frames at the same timestamp, the script calculates the "True Sync Offset" in milliseconds.

### Outlier Removal
To ensure accuracy, the tool uses the **Interquartile Range (IQR)** method to filter out noise. If the clock detection fails or is blurry in a specific frame, that data point is discarded to prevent it from skewing the final average.

## Requirements
- Python 3
- OpenCV (`opencv-python`)
- NumPy

Install dependencies via pip:
```bash
pip install opencv-python numpy
```

## Video Format
The bulk analyser expects video pairs to follow the naming convention:
- `left_YYYYMMDD_HHMMSS.mkv`
- `right_YYYYMMDD_HHMMSS.mkv`
