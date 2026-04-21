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

def find_largest_centroid(mask, min_area=20):
    """Finds the centroid of the largest contour in a mask if its area > min_area."""
    contours, _ = cv2.findContours(mask, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
    if not contours:
        return None, None
        
    largest = max(contours, key=cv2.contourArea)
    if cv2.contourArea(largest) < min_area:
        return None, None
        
    M = cv2.moments(largest)
    if M["m00"] > 0:
        center = (int(M["m10"] / M["m00"]), int(M["m01"] / M["m00"]))
        return center, largest
    return None, None

def find_and_warp_marker_box(frame, hsv_frame, lower_color, upper_color, width, height, min_area=30, marker_draw_color=None):
    """
    Finds 4 markers of a specific color and warps the area between them to a flat (width x height) rectangle.
    Returns the warped image if successful, otherwise None.
    """
    mask = cv2.inRange(hsv_frame, lower_color, upper_color)
    
    # Use opening to remove small noise spots.
    kernel = np.ones((5, 5), np.uint8)
    mask = cv2.morphologyEx(mask, cv2.MORPH_OPEN, kernel)
    
    contours, _ = cv2.findContours(mask, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
    
    potential_corners = []
    for c in contours:
        area = cv2.contourArea(c)
        if area > min_area:
            rect = cv2.minAreaRect(c)
            (w, h) = rect[1]
            if w > 0 and h > 0:
                aspect_ratio = max(w, h) / min(w, h)
                if aspect_ratio < 2.5: # must be roughly square/round
                    potential_corners.append(c)
    
    if len(potential_corners) < 4:
        return None

    # Take the 4 "best" candidates (largest ones)
    potential_corners = sorted(potential_corners, key=cv2.contourArea, reverse=True)[:4]
    
    corner_pts = []
    for c in potential_corners:
        M = cv2.moments(c)
        if M["m00"] > 0:
            cx, cy = int(M["m10"] / M["m00"]), int(M["m01"] / M["m00"])
            corner_pts.append([cx, cy])
            if marker_draw_color is not None:
                cv2.drawMarker(frame, (cx, cy), marker_draw_color, cv2.MARKER_SQUARE, 10, 2)
            
    if len(corner_pts) < 4:
        return None

    corner_pts = np.array(corner_pts, dtype="float32")
    
    # Check if any points are too close together
    min_dist = 50 
    for i in range(4):
        for j in range(i + 1, 4):
            dist = np.linalg.norm(corner_pts[i] - corner_pts[j])
            if dist < min_dist:
                return None

    ordered_pts = order_points(corner_pts)
    
    dst_pts = np.array([[0, 0], [width-1, 0], [width-1, height-1], [0, height-1]], dtype="float32")
    matrix = cv2.getPerspectiveTransform(ordered_pts, dst_pts)
    warped = cv2.warpPerspective(frame, matrix, (width, height))
    
    return warped

def extract_text_box(frame, hsv_frame):
    """Finds the 4 blue corners and warps the text area to a flat 600x200 rectangle."""
    lower_blue = np.array([100, 150, 50])
    upper_blue = np.array([135, 255, 255])
    
    flat_text = find_and_warp_marker_box(frame, hsv_frame, lower_blue, upper_blue, 600, 200, min_area=50, marker_draw_color=(255, 0, 0))
    
    if flat_text is None:
        return np.zeros((200, 600, 3), dtype=np.uint8)

    # Draw a blue border around it
    cv2.rectangle(flat_text, (0,0), (599,199), (255,0,0), 4)
    return flat_text

def extract_clock_box(frame, hsv_frame):
    """Finds the 4 green corners and warps the clock area to a flat 600x600 rectangle."""
    lower_green = np.array([40, 100, 100])
    upper_green = np.array([80, 255, 255])
    
    flat_clock = find_and_warp_marker_box(frame, hsv_frame, lower_green, upper_green, 600, 600, min_area=30)
    
    if flat_clock is None:
        blank_clock = cv2.resize(frame, (600, 600))
        cv2.putText(blank_clock, "Waiting for 4 green corners...", (50, 50), cv2.FONT_HERSHEY_SIMPLEX, 1, (0, 0, 255), 2)
        return False, blank_clock

    return True, flat_clock

def find_central_anchor(clock, clock_hsv):
    """Finds the central red anchor point."""
    lower_red1, upper_red1 = np.array([0, 120, 70]), np.array([10, 255, 255])
    lower_red2, upper_red2 = np.array([170, 120, 70]), np.array([180, 255, 255])
    mask_red = cv2.bitwise_or(cv2.inRange(clock_hsv, lower_red1, upper_red1), 
                              cv2.inRange(clock_hsv, lower_red2, upper_red2))
    
    anchor_pt, anchor_contour = find_largest_centroid(mask_red, min_area=20)
    if anchor_pt:
        cv2.drawContours(clock, [anchor_contour], -1, (255, 0, 0), 2)
        cv2.drawMarker(clock, anchor_pt, (0, 255, 0), cv2.MARKER_CROSS, 20, 2)
    return anchor_pt

def find_hand(clock):
    """Finds the white clock hand."""
    gray = cv2.cvtColor(clock, cv2.COLOR_BGR2GRAY)
    _, mask_white = cv2.threshold(gray, DOT_THRESHOLD, 255, cv2.THRESH_BINARY)
    
    dot_pt, dot_contour = find_largest_centroid(mask_white, min_area=50)
    if dot_pt:
        cv2.drawContours(clock, [dot_contour], -1, (255, 0, 255), 2)
        cv2.drawMarker(clock, dot_pt, (255, 0, 0), cv2.MARKER_CROSS, 20, 2)
    return dot_pt

def calculate_angle(anchor_pt, hand_pt):
    """Calculates the angle of the clock hand in degrees."""
    dy = hand_pt[1] - anchor_pt[1]
    dx = hand_pt[0] - anchor_pt[0]
    angle_rad = math.atan2(dy, dx)
    angle_deg = math.degrees(angle_rad)
    if angle_deg < 0: angle_deg += 360
    return angle_deg

def flatten_and_find_features(frame):
    hsv = cv2.cvtColor(frame, cv2.COLOR_BGR2HSV)
    
    textbox = extract_text_box(frame, hsv)
    
    found_clock, clock = extract_clock_box(frame, hsv)
    
    if not found_clock:
        return None, np.vstack((clock, textbox))
    
    clock_hsv = cv2.cvtColor(clock, cv2.COLOR_BGR2HSV)
    
    # Locate red anchor point (center of clock)
    anchor_pt = find_central_anchor(clock, clock_hsv)
    
    # Locate clock hand
    hand_pt = find_hand(clock)

    angle_deg = None
    if anchor_pt and hand_pt:
        cv2.line(clock, anchor_pt, hand_pt, (0, 255, 255), 2)
        angle_deg = calculate_angle(anchor_pt, hand_pt)
        cv2.putText(clock, f"{angle_deg:.1f} deg", (hand_pt[0] + 10, hand_pt[1]), 
                    cv2.FONT_HERSHEY_SIMPLEX, 0.7, (0, 255, 255), 2)

    cv2.rectangle(clock, (0,0), (599,599), (0,255,0), 4)

    # Stack the clock and text together
    final_column = np.vstack((clock, textbox))
    
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
        # Ensure that the two videos are roughly in sync using the PTS.
        # If they aren't, skip the frame that is behind.
        ts_l = left_cap.get(cv2.CAP_PROP_POS_MSEC)
        ts_r = right_cap.get(cv2.CAP_PROP_POS_MSEC)
        
        if ts_l < ts_r - MAX_TIMECODE_DIFF:
            print(f"Skipping left frame {ts_l} (too far behind right frame {ts_r})")
            ret_l, frame_l = left_cap.read()
            continue
        elif ts_r < ts_l - MAX_TIMECODE_DIFF:
            print(f"Skipping right frame {ts_r} (too far behind left frame {ts_l})")
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