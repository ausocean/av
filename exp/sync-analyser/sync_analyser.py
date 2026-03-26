import cv2
import numpy as np
import math
import argparse

# --- CONFIGURATION ---
ROTATION_TIME_MS = 1000.0  
MAX_TIMECODE_DIFF = 10.0   
DOT_THRESHOLD = 100 # Change this if the white dot is not detected.

def order_points(pts):
    """Sorts 4 points into: Top-Left, Top-Right, Bottom-Right, Bottom-Left."""
    rect = np.zeros((4, 2), dtype="float32")
    s = pts.sum(axis=1)
    rect[0] = pts[np.argmin(s)] 
    rect[2] = pts[np.argmax(s)] 
    
    diff = np.diff(pts, axis=1) 
    rect[1] = pts[np.argmin(diff)] 
    rect[3] = pts[np.argmax(diff)] 
    return rect

def extract_text_box(frame, hsv_frame):
    """Finds the 4 blue corners and warps the text area to a flat 600x200 rectangle."""
    lower_blue = np.array([100, 150, 50])
    upper_blue = np.array([135, 255, 255])
    mask_blue = cv2.inRange(hsv_frame, lower_blue, upper_blue)
    
    # Use opening to remove small noise spots.
    kernel = np.ones((5, 5), np.uint8)
    mask_blue = cv2.morphologyEx(mask_blue, cv2.MORPH_OPEN, kernel)
    
    contours_blue, _ = cv2.findContours(mask_blue, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
    
    # Filter for markers that are:
    # 1. Broadly the right size (not too small, not like huge background objects)
    # 2. Relatively compact (like a dot or square, not a long line or fuzzy cloud)
    potential_corners = []
    for c in contours_blue:
        area = cv2.contourArea(c)
        if 50 < area < 3000: # markers are usually small blobs
            rect = cv2.minAreaRect(c)
            (w, h) = rect[1]
            if w > 0 and h > 0:
                aspect_ratio = max(w, h) / min(w, h)
                if aspect_ratio < 2.5: # must be roughly square/round
                    potential_corners.append(c)
    
    # Return a blank black box if we don't have at least 4 candidates
    if len(potential_corners) < 4:
        return np.zeros((200, 600, 3), dtype=np.uint8)

    # Take the 4 "best" candidates (closest to the average area of candidates)
    # This helps if we have 4 markers and some smaller/larger noise
    potential_corners = sorted(potential_corners, key=cv2.contourArea, reverse=True)[:4]
    
    corner_pts = []
    for c in potential_corners:
        M = cv2.moments(c)
        if M["m00"] > 0:
            cx, cy = int(M["m10"] / M["m00"]), int(M["m01"] / M["m00"])
            corner_pts.append([cx, cy])
            cv2.drawMarker(frame, (cx, cy), (255, 0, 0), cv2.MARKER_SQUARE, 10, 2)
            
    if len(corner_pts) < 4:
        return np.zeros((200, 600, 3), dtype=np.uint8)

    corner_pts = np.array(corner_pts, dtype="float32")
    
    # Check if any points are too close together
    min_dist = 50 
    for i in range(4):
        for j in range(i + 1, 4):
            dist = np.linalg.norm(corner_pts[i] - corner_pts[j])
            if dist < min_dist:
                return np.zeros((200, 600, 3), dtype=np.uint8)

    ordered_pts = order_points(corner_pts)
    
    # Stretch the text box to 600x200
    dst_pts = np.array([[0, 0], [599, 0], [599, 199], [0, 199]], dtype="float32")
    matrix = cv2.getPerspectiveTransform(ordered_pts, dst_pts)
    flat_text = cv2.warpPerspective(frame, matrix, (600, 200))
    
    # Draw a blue border around it for style
    cv2.rectangle(flat_text, (0,0), (599,199), (255,0,0), 4)
    return flat_text

def flatten_and_find_features(frame):
    hsv = cv2.cvtColor(frame, cv2.COLOR_BGR2HSV)
    
    # --- 1. EXTRACT THE TEXT BOX ---
    flat_text = extract_text_box(frame, hsv)
    
    # --- 2. EXTRACT THE CLOCK BOX ---
    lower_green = np.array([40, 100, 100])
    upper_green = np.array([80, 255, 255])
    mask_green = cv2.inRange(hsv, lower_green, upper_green)
    
    contours_green, _ = cv2.findContours(mask_green, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
    valid_corners = [c for c in contours_green if cv2.contourArea(c) > 30]
    
    if len(valid_corners) < 4:
        blank_clock = cv2.resize(frame, (600, 600))
        cv2.putText(blank_clock, "Waiting for 4 green corners...", (50, 50), cv2.FONT_HERSHEY_SIMPLEX, 1, (0, 0, 255), 2)
        return None, np.vstack((blank_clock, flat_text))

    valid_corners = sorted(valid_corners, key=cv2.contourArea, reverse=True)[:4]
    corner_pts = []
    for c in valid_corners:
        M = cv2.moments(c)
        if M["m00"] > 0:
            corner_pts.append([int(M["m10"]/M["m00"]), int(M["m01"]/M["m00"])])
            
    corner_pts = np.array(corner_pts, dtype="float32")
    ordered_pts = order_points(corner_pts)
    
    dst_pts = np.array([[0, 0], [599, 0], [599, 599], [0, 599]], dtype="float32")
    matrix = cv2.getPerspectiveTransform(ordered_pts, dst_pts)
    flat_clock = cv2.warpPerspective(frame, matrix, (600, 600))
    
    # --- 3. DO THE MATH ON THE FLAT CLOCK ---
    flat_hsv = cv2.cvtColor(flat_clock, cv2.COLOR_BGR2HSV)
    
    lower_red1, upper_red1 = np.array([0, 120, 70]), np.array([10, 255, 255])
    lower_red2, upper_red2 = np.array([170, 120, 70]), np.array([180, 255, 255])
    mask_red = cv2.bitwise_or(cv2.inRange(flat_hsv, lower_red1, upper_red1), 
                              cv2.inRange(flat_hsv, lower_red2, upper_red2))
    
    contours_red, _ = cv2.findContours(mask_red, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
    anchor_pt = None
    if contours_red:
        largest_red = max(contours_red, key=cv2.contourArea)
        if cv2.contourArea(largest_red) > 20: 
            M_red = cv2.moments(largest_red)
            if M_red["m00"] > 0:
                anchor_pt = (int(M_red["m10"]/M_red["m00"]), int(M_red["m01"]/M_red["m00"]))
                cv2.drawMarker(flat_clock, anchor_pt, (0, 255, 0), cv2.MARKER_CROSS, 20, 2)

    gray = cv2.cvtColor(flat_clock, cv2.COLOR_BGR2GRAY)
    _, mask_white = cv2.threshold(gray, DOT_THRESHOLD, 255, cv2.THRESH_BINARY)
    contours_white, _ = cv2.findContours(mask_white, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
    
    dot_pt = None
    if contours_white:
        largest_white = max(contours_white, key=cv2.contourArea)
        cv2.drawContours(flat_clock, [largest_white], -1, (255, 0, 255), 2)
        if cv2.contourArea(largest_white) > 50: 
            M_white = cv2.moments(largest_white)
            if M_white["m00"] > 0:
                dot_pt = (int(M_white["m10"]/M_white["m00"]), int(M_white["m01"]/M_white["m00"]))
                cv2.drawMarker(flat_clock, dot_pt, (255, 0, 0), cv2.MARKER_CROSS, 20, 2)

    angle_deg = None
    if anchor_pt and dot_pt:
        cv2.line(flat_clock, anchor_pt, dot_pt, (0, 255, 255), 2)
        dy = dot_pt[1] - anchor_pt[1]
        dx = dot_pt[0] - anchor_pt[0]
        angle_rad = math.atan2(dy, dx)
        angle_deg = math.degrees(angle_rad)
        if angle_deg < 0: angle_deg += 360
        cv2.putText(flat_clock, f"{angle_deg:.1f} deg", (dot_pt[0] + 10, dot_pt[1]), 
                    cv2.FONT_HERSHEY_SIMPLEX, 0.7, (0, 255, 255), 2)

    cv2.rectangle(flat_clock, (0,0), (599,599), (0,255,0), 4)

    # --- 4. STACK THE CLOCK AND TEXT TOGETHER ---
    final_column = np.vstack((flat_clock, flat_text))
    
    return angle_deg, final_column

def calculate_time_offset(angle_left, angle_right):
    diff = angle_right - angle_left
    if diff > 180: diff -= 360
    elif diff < -180: diff += 360
    return (diff / 360.0) * ROTATION_TIME_MS

def remove_outliers(data):
    """Removes outliers using the Interquartile Range (IQR) method."""
    if not data:
        return []
    q1 = np.percentile(data, 25)
    q3 = np.percentile(data, 75)
    iqr = q3 - q1
    lower_bound = q1 - 1.5 * iqr
    upper_bound = q3 + 1.5 * iqr
    return [x for x in data if lower_bound <= x <= upper_bound]

def analyze_sync(left_vid_path, right_vid_path, headless=False):
    left_cap = cv2.VideoCapture(left_vid_path)
    right_cap = cv2.VideoCapture(right_vid_path)
    
    ret_l, frame_l = left_cap.read()
    ret_r, frame_r = right_cap.read()
    
    offsets = []
    paused = False
    
    while ret_l and ret_r:
        ts_l = left_cap.get(cv2.CAP_PROP_POS_MSEC)
        ts_r = right_cap.get(cv2.CAP_PROP_POS_MSEC)
        
        if ts_l < ts_r - MAX_TIMECODE_DIFF:
            ret_l, frame_l = left_cap.read()
            continue
        elif ts_r < ts_l - MAX_TIMECODE_DIFF:
            ret_r, frame_r = right_cap.read()
            continue
            
        angle_l, debug_l = flatten_and_find_features(frame_l)
        angle_r, debug_r = flatten_and_find_features(frame_r)
        
        physical_ms_offset = 0.0
        
        if angle_l is not None and angle_r is not None:
            physical_ms_offset = calculate_time_offset(angle_l, angle_r)
            offsets.append(physical_ms_offset)
            if not paused and not headless:
                print(f"Angle L: {angle_l:.1f} | Angle R: {angle_r:.1f} -> True Sync: {physical_ms_offset:+.2f}ms")
        
        if not headless:
            combined_view = np.hstack((debug_l, debug_r))
            status = "PAUSED" if paused else "PLAYING"
            cv2.putText(combined_view, f"Sync Offset: {physical_ms_offset:+.2f} ms | [{status}]", 
                        (20, 40), cv2.FONT_HERSHEY_SIMPLEX, 1.0, (255, 255, 255), 3)
            
            cv2.imshow("Stereo Sync Analyzer", combined_view)
            
            while True:
                delay = 0 if paused else 1
                key = cv2.waitKey(delay) & 0xFF
                if key == ord('q'):
                    left_cap.release()
                    right_cap.release()
                    cv2.destroyAllWindows()
                    return None
                elif key == ord(' ') or key == ord('p'):
                    paused = not paused
                    break 
                elif key == ord('n') and paused:
                    break 
                elif not paused:
                    break 

        ret_l, frame_l = left_cap.read()
        ret_r, frame_r = right_cap.read()

    left_cap.release()
    right_cap.release()
    if not headless:
        cv2.destroyAllWindows()
    
    filtered_offsets = remove_outliers(offsets)
    
    results = {
        "raw_offsets": offsets,
        "filtered_offsets": filtered_offsets,
        "count": len(offsets),
        "filtered_count": len(filtered_offsets),
        "mean": np.mean(filtered_offsets) if filtered_offsets else 0.0,
        "std": np.std(filtered_offsets) if filtered_offsets else 0.0
    }
    
    if not headless and offsets:
        print("\n--- FINAL RESULTS ---")
        print(f"Frames Analyzed: {results['count']}")
        print(f"Filtered Frames: {results['filtered_count']}")
        print(f"Average True Sync Offset (Filtered): {results['mean']:+.2f} ms")
        print(f"Jitter (Std Dev Filtered): {results['std']:.2f} ms")
        
    return results

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Analyze synchronization between two stereo cameras.")
    parser.add_argument("left_video", help="Path to the left camera video file.")
    parser.add_argument("right_video", help="Path to the right camera video file.")
    args = parser.parse_args()
    
    analyze_sync(args.left_video, args.right_video)