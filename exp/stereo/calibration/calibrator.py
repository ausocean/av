import glob
import os
import re
import cv2 as cv
import numpy as np
import yaml
import argparse
import logging

def extract_timestamp_from_filename(filepath):
    """Extract timestamp in seconds from a filename like frame_NNNNNN_TTTT.TTTs.png."""
    match = re.search(r'_(\d+(?:\.\d+)?)s\.png$', os.path.basename(filepath))
    return float(match.group(1)) if match else None


def match_stereo_pairs_by_timestamp(imagesLeft, imagesRight, max_diff_seconds=2.0):
    """
    Match left and right image paths by closest timestamp within tolerance.
    Falls back to index-based zip if timestamps cannot be parsed.
    Returns two parallel lists: matched_left, matched_right.
    """
    left_ts = [(extract_timestamp_from_filename(p), p) for p in imagesLeft]
    right_ts = [(extract_timestamp_from_filename(p), p) for p in imagesRight]

    if any(t is None for t, _ in left_ts) or any(t is None for t, _ in right_ts):
        logging.warning("Could not parse timestamps from filenames; falling back to index-based pairing.")
        return list(imagesLeft), list(imagesRight)

    matched_left, matched_right = [], []
    used_right = set()
    for t_left, left_path in left_ts:
        best_idx, best_diff = None, float('inf')
        for i, (t_right, _) in enumerate(right_ts):
            if i in used_right:
                continue
            diff = abs(t_right - t_left)
            if diff < best_diff:
                best_diff = diff
                best_idx = i
        if best_idx is not None and best_diff <= max_diff_seconds:
            t_right, right_path = right_ts[best_idx]
            matched_left.append(left_path)
            matched_right.append(right_path)
            used_right.add(best_idx)
            logging.debug(f"Matched: {os.path.basename(left_path)} ({t_left:.3f}s) <-> "
                          f"{os.path.basename(right_path)} ({t_right:.3f}s), diff={best_diff:.3f}s")
        else:
            logging.debug(f"No right match within {max_diff_seconds}s for {os.path.basename(left_path)} ({t_left:.3f}s)")

    logging.info(f"Timestamp-matched {len(matched_left)} stereo pairs "
                 f"(from {len(imagesLeft)} left, {len(imagesRight)} right images).")
    return matched_left, matched_right


def print_map_stats(stereoMapL, stereoMapR):
    """
    Print statistics for stereo rectification maps.
    Args:
        stereoMapL (tuple[np.ndarray, np.ndarray]): Left rectification maps.
        stereoMapR (tuple[np.ndarray, np.ndarray]): Right rectification maps.
    Returns:
        None
    """
    print("Left Map X: min=", np.min(stereoMapL[0]), "max=", np.max(stereoMapL[0]), "mean=", np.mean(stereoMapL[0]))
    print("Left Map Y: min=", np.min(stereoMapL[1]), "max=", np.max(stereoMapL[1]), "mean=", np.mean(stereoMapL[1]))
    print("Right Map X: min=", np.min(stereoMapR[0]), "max=", np.max(stereoMapR[0]), "mean=", np.mean(stereoMapR[0]))
    print("Right Map Y: min=", np.min(stereoMapR[1]), "max=", np.max(stereoMapR[1]), "mean=", np.mean(stereoMapR[1]))

def load_config(config_file_path):
    """
    Load configuration from a YAML file.
    Args:
        config_file_path (str): Path to the YAML configuration file.
    Returns:
        dict: Configuration parameters loaded from the YAML file.
    """
    with open(config_file_path, 'r') as file:
        config_data = yaml.safe_load(file)
    return config_data

def setup_logging(config):
    log_level = config.get('logging_level', 'INFO').upper()
    log_format = '%(asctime)s - %(levelname)s - %(message)s'
    logging.basicConfig(level=getattr(logging, log_level, logging.INFO), format=log_format)
    logging.info(f"Logging initialized with level: {log_level}")

def read_timecodes(timecode_file):
    """Reads a v2 timecode file and returns a list of timestamps in milliseconds."""
    timecodes = []
    with open(timecode_file, 'r') as f:
        for line in f:
            line = line.strip()
            # Skip empty lines and the header
            if not line or line.startswith('#'):
                continue
            timecodes.append(float(line))
    return timecodes

def extract_frames_from_h264_vfr(h264_file, timecode_file, output_dir, interval_seconds):
    """
    Extract frames using exact timecodes to ensure VFR sync.
    """
    if not os.path.exists(output_dir):
        os.makedirs(output_dir)
    logging.info(f"Extracting frames from {h264_file} using timecodes from {timecode_file} into {output_dir}.")
        
    cap = cv.VideoCapture(h264_file)
    if not cap.isOpened():
        logging.error(f"Failed to open video file: {h264_file}")
        raise RuntimeError(f"Failed to open video file: {h264_file}")
    
    timecodes = read_timecodes(timecode_file)
    logging.debug(f"Loaded {len(timecodes)} timecodes.")
    interval_ms = interval_seconds * 1000.0  # Convert target interval to milliseconds
    target_ms = 0.0
    
    frame_idx = 0
    extracted_count = 0
    
    while True:
        ret, frame = cap.read()
        if not ret:
            break
        if frame_idx >= len(timecodes):
            break
        current_ms = timecodes[frame_idx]
        if current_ms >= target_ms:
            timestamp_sec = current_ms / 1000.0
            output_file = os.path.join(output_dir, f"frame_{extracted_count:06d}_{timestamp_sec:.3f}s.png")
            cv.imwrite(output_file, frame)
            logging.debug(f"Extracted frame {extracted_count} at {timestamp_sec:.3f}s to {output_file}")
            extracted_count += 1
            target_ms += interval_ms
        frame_idx += 1
    cap.release()
    logging.info(f"Extracted {extracted_count} frames from {h264_file}.")
    return extracted_count

def find_chessboard_corners(imagesLeft, imagesRight, chessboard_size, criteria, checkerboard_squares_size):
    """
    Find chessboard corners in stereo image pairs.
    Args:
        imagesLeft (list[str]): List of left image file paths.
        imagesRight (list[str]): List of right image file paths.
        chessboard_size (tuple[int, int]): Number of inner corners in the chessboard pattern.
        criteria (tuple): Termination criteria for cornerSubPix.
        checkerboard_squares_size (float): Size of a checkerboard square in real-world units.
    Returns:
        objpoints (list[np.ndarray]): 3D object points in real world space.
        imgpointsL (list[np.ndarray]): 2D image points for left images.
        imgpointsR (list[np.ndarray]): 2D image points for right images.
        frame_size (tuple[int, int]): Image size (width, height).
    """
    objp = np.zeros((chessboard_size[0] * chessboard_size[1], 3), np.float32)
    objp[:,:2] = np.mgrid[0:chessboard_size[0],0:chessboard_size[1]].T.reshape(-1,2) * checkerboard_squares_size
    objpoints = []
    imgpointsL = []
    imgpointsR = []
    frame_size = None
    logging.info(f"Finding chessboard corners in {len(imagesLeft)} left and {len(imagesRight)} right images.")
    for idx, (imgLeft, imgRight) in enumerate(zip(imagesLeft, imagesRight)):
        imgL = cv.imread(imgLeft)
        imgR = cv.imread(imgRight)
        if imgL is None or imgR is None:
            logging.error(f"Failed to read calibration pair: {imgLeft}, {imgRight}")
            raise RuntimeError(f"Failed to read calibration pair: {imgLeft}, {imgRight}")
        grayL = cv.cvtColor(imgL, cv.COLOR_BGR2GRAY)
        grayR = cv.cvtColor(imgR, cv.COLOR_BGR2GRAY)
        if grayL.shape != grayR.shape:
            logging.error("Left/right calibration images must have the same resolution.")
            raise RuntimeError("Left/right calibration images must have the same resolution.")
        if frame_size is None:
            frame_size = grayL.shape[::-1]
        elif frame_size != grayL.shape[::-1]:
            logging.error("Calibration images must all have the same resolution.")
            raise RuntimeError("Calibration images must all have the same resolution.")
       
        find_flags = cv.CALIB_CB_ADAPTIVE_THRESH + cv.CALIB_CB_NORMALIZE_IMAGE + cv.CALIB_CB_FAST_CHECK
        retL, cornersL = cv.findChessboardCorners(grayL, chessboard_size, flags=find_flags)
        retR, cornersR = cv.findChessboardCorners(grayR, chessboard_size, flags=find_flags)
        if retL and retR:
            objpoints.append(objp)
            cornersL = cv.cornerSubPix(grayL, cornersL, (11,11), (-1,-1), criteria)
            imgpointsL.append(cornersL)
            cornersR = cv.cornerSubPix(grayR, cornersR, (11,11), (-1,-1), criteria)
            imgpointsR.append(cornersR)
            logging.debug(f"Chessboard found in pair {idx}: {imgLeft}, {imgRight}")
        else:
            logging.warning(f"Chessboard not found in pair {idx}: {imgLeft}, {imgRight}")
    cv.destroyAllWindows()
    logging.info(f"Found corners in {len(objpoints)} pairs.")
    return objpoints, imgpointsL, imgpointsR, frame_size

def calibrate_cameras(objpoints, imgpointsL, imgpointsR, frame_size):
    """
    Calibrate left and right cameras individually.
    Args:
        objpoints (list[np.ndarray]): 3D object points in real world space.
        imgpointsL (list[np.ndarray]): 2D image points for left images.
        imgpointsR (list[np.ndarray]): 2D image points for right images.
        frame_size (tuple[int, int]): Image size (width, height).
    Returns:
        cameraMatrixL (np.ndarray): Left camera intrinsic matrix.
        distL (np.ndarray): Left camera distortion coefficients.
        roi_L (tuple): Left ROI.
        cameraMatrixR (np.ndarray): Right camera intrinsic matrix.
        distR (np.ndarray): Right camera distortion coefficients.
        roi_R (tuple): Right ROI.
    """
    # Check for fisheye calibration
    logging.info(f"Calibrating cameras with {len(objpoints)} valid pairs.")
    if config.get('fisheye', False):
        K1 = np.zeros((3, 3))
        D1 = np.zeros((4, 1))
        K2 = np.zeros((3, 3))
        D2 = np.zeros((4, 1))
        objpoints_fisheye = [op.reshape(-1, 1, 3) for op in objpoints]
        imgpointsL_fisheye = [ip.reshape(-1, 1, 2) for ip in imgpointsL]
        imgpointsR_fisheye = [ip.reshape(-1, 1, 2) for ip in imgpointsR]
        logging.debug(f"Fisheye calibration: {len(objpoints_fisheye)} pairs.")
        cv.fisheye.calibrate(objpoints_fisheye, imgpointsL_fisheye, frame_size, K1, D1, None, None, flags=cv.fisheye.CALIB_RECOMPUTE_EXTRINSIC)
        cv.fisheye.calibrate(objpoints_fisheye, imgpointsR_fisheye, frame_size, K2, D2, None, None, flags=cv.fisheye.CALIB_RECOMPUTE_EXTRINSIC)
        return (K1, D1, K1, None, K2, D2, K2, None)
    else:
        retL, cameraMatrixL, distL, rvecsL, tvecsL = cv.calibrateCamera(objpoints, imgpointsL, frame_size, None, None)
        retR, cameraMatrixR, distR, rvecsR, tvecsR = cv.calibrateCamera(objpoints, imgpointsR, frame_size, None, None)
        logging.info(f"Left camera RMS error: {retL}")
        logging.info(f"Right camera RMS error: {retR}")
        logging.debug(f"Left camera matrix:\n{cameraMatrixL}\nDistortion:\n{distL}")
        logging.debug(f"Right camera matrix:\n{cameraMatrixR}\nDistortion:\n{distR}")
        return (cameraMatrixL, distL, None,
                cameraMatrixR, distR, None)

def calibrate_stereo_camera(objpoints, imgpointsL, imgpointsR, cameraMatrixL, distL, cameraMatrixR, distR, frame_size):
    """
    Calibrate stereo camera system.
    Args:
        objpoints (list[np.ndarray]): 3D object points in real world space.
        imgpointsL (list[np.ndarray]): 2D image points for left images.
        imgpointsR (list[np.ndarray]): 2D image points for right images.
        newCameraMatrixL (np.ndarray): Left camera intrinsic matrix.
        distL (np.ndarray): Left camera distortion coefficients.
        newCameraMatrixR (np.ndarray): Right camera intrinsic matrix.
        distR (np.ndarray): Right camera distortion coefficients.
        frame_size (tuple[int, int]): Image size (width, height).
    Returns:
        rot (np.ndarray): Rotation matrix between cameras.
        trans (np.ndarray): Translation vector between cameras.
        essentialMatrix (np.ndarray): Essential matrix.
        fundamentalMatrix (np.ndarray): Fundamental matrix.
    """
    if config.get('fisheye', False):
        # Fisheye calibration
        K1 = np.zeros((3, 3))
        D1 = np.zeros((4, 1))
        K2 = np.zeros((3, 3))
        D2 = np.zeros((4, 1))
        objpoints_fisheye = [op.reshape(-1, 1, 3) for op in objpoints]
        imgpointsL_fisheye = [ip.reshape(-1, 1, 2) for ip in imgpointsL]
        imgpointsR_fisheye = [ip.reshape(-1, 1, 2) for ip in imgpointsR]
        cv.fisheye.calibrate(objpoints_fisheye, imgpointsL_fisheye, frame_size, K1, D1, None, None, flags=cv.fisheye.CALIB_RECOMPUTE_EXTRINSIC)
        cv.fisheye.calibrate(objpoints_fisheye, imgpointsR_fisheye, frame_size, K2, D2, None, None, flags=cv.fisheye.CALIB_RECOMPUTE_EXTRINSIC)
        return (K1, D1, K2, D2)
    else:
        flags = cv.CALIB_FIX_INTRINSIC
        criteria_stereo = (cv.TERM_CRITERIA_EPS + cv.TERM_CRITERIA_MAX_ITER, 100, 1e-6)
        
        retStereo, cameraMatrixL_out, distL_out, cameraMatrixR_out, distR_out, rot, trans, essentialMatrix, fundamentalMatrix = cv.stereoCalibrate(
            objpoints,
            imgpointsL,
            imgpointsR,
            cameraMatrixL,
            distL,
            cameraMatrixR,
            distR,
            frame_size,
            flags=flags,
            criteria=criteria_stereo
        )
        logging.info(f"Stereo calibration RMS error: {retStereo}")
        logging.info(f"Refined Left Camera Matrix:\n{cameraMatrixL_out}\nDistortion:\n{distL_out}")
        logging.info(f"Refined Right Camera Matrix:\n{cameraMatrixR_out}\nDistortion:\n{distR_out}")
        return rot, trans, essentialMatrix, fundamentalMatrix, cameraMatrixL_out, distL_out, cameraMatrixR_out, distR_out

def compute_rectification_maps(cameraMatrixL, distL, cameraMatrixR, distR, frame_size, rot, trans, rectify_scale=1):
    if config.get('fisheye', False):
        R1, R2, P1, P2, Q = cv.fisheye.stereoRectify(
            cameraMatrixL,
            distL,
            cameraMatrixR,
            distR,
            frame_size,
            rot,
            trans,
            flags=cv.fisheye.CALIB_ZERO_DISPARITY,
            newImageSize=frame_size,
            balance=rectify_scale,
            fov_scale=1.0
        )

        stereoMapL = cv.fisheye.initUndistortRectifyMap(
            cameraMatrixL, distL, R1, P1, frame_size, cv.CV_32FC1
        )
        stereoMapR = cv.fisheye.initUndistortRectifyMap(
            cameraMatrixR, distR, R2, P2, frame_size, cv.CV_32FC1
        )
        roi_L, roi_R = None, None
        return stereoMapL, stereoMapR, P1, P2, Q, roi_L, roi_R

    rectL, rectR, projMatrixL, projMatrixR, Q, roi_L, roi_R = cv.stereoRectify(
        cameraMatrix1=cameraMatrixL,
        distCoeffs1=distL,
        cameraMatrix2=cameraMatrixR,
        distCoeffs2=distR,
        imageSize=frame_size,
        R=rot,
        T=trans,
        flags=cv.CALIB_ZERO_DISPARITY,
        alpha=rectify_scale,
        newImageSize=frame_size
    )

    stereoMapL = cv.initUndistortRectifyMap(
        cameraMatrixL, distL, rectL, projMatrixL, frame_size, cv.CV_32FC1
    )
    stereoMapR = cv.initUndistortRectifyMap(
        cameraMatrixR, distR, rectR, projMatrixR, frame_size, cv.CV_32FC1
    )
    return stereoMapL, stereoMapR, projMatrixL, projMatrixR, Q, roi_L, roi_R

def save_stereo_maps(stereoMapL, stereoMapR, output_path):
    """
    Save stereo rectification maps to an XML file.
    Args:
        stereoMapL (tuple[np.ndarray, np.ndarray]): Left rectification maps.
        stereoMapR (tuple[np.ndarray, np.ndarray]): Right rectification maps.
        output_path (str): Path to output XML file.
    Returns:
        None
    """
    logging.info(f"Saving stereo maps to {output_path} ...")
    map_bytes = sum(m.nbytes for m in [stereoMapL[0], stereoMapL[1], stereoMapR[0], stereoMapR[1]])
    logging.info(f"Total map data size: {map_bytes / 1024 / 1024:.1f} MB (4 arrays of shape {stereoMapL[0].shape}, dtype {stereoMapL[0].dtype})")

    logging.debug("Opening FileStorage for writing ...")
    cv_file = cv.FileStorage(output_path, cv.FILE_STORAGE_WRITE)

    logging.debug("Writing stereoMapL_x ...")
    cv_file.write('stereoMapL_x', stereoMapL[0])

    logging.debug("Writing stereoMapL_y ...")
    cv_file.write('stereoMapL_y', stereoMapL[1])

    logging.debug("Writing stereoMapR_x ...")
    cv_file.write('stereoMapR_x', stereoMapR[0])

    logging.debug("Writing stereoMapR_y ...")
    cv_file.write('stereoMapR_y', stereoMapR[1])

    logging.debug("Flushing and closing file ...")
    cv_file.release()
    logging.info(f"Stereo maps saved successfully to {output_path}")

def main():
    parser = argparse.ArgumentParser(
        prog='ProgramName',
        description='Stereo calibration tool',
        epilog='Text at the bottom of help')
    parser.add_argument('video_directory')
    parser.add_argument('output_directory')
    parser.add_argument('--use_existing_images', action='store_true', help='Use existing PNG images instead of extracting frames from h264 videos')
    parser.add_argument('--use_stored_calibration', action='store_true', help='Use stored camera calibration parameters from camera_calibration.npz')
    args = parser.parse_args()
    print(args.video_directory, args.output_directory)

    global config
    config = load_config('config.yaml')
    setup_logging(config)
    CHESSBOARD_SIZE = tuple(config['chessboard_size']) 
    INTERVAL_SECONDS = config['interval_seconds']
    CHECKERBOARD_SQUARES_SIZE = config['checkerboard_squares_size']
    RECORDING_OFFSET = config.get('recording_offset', 0)
    FISHEYE = config.get('fisheye', False)

    logging.info(f"Chessboard size: {CHESSBOARD_SIZE}, Fisheye: {FISHEYE}")

    CRITERIA = (cv.TERM_CRITERIA_EPS + cv.TERM_CRITERIA_MAX_ITER, 30, 0.001)

    video_dir = args.video_directory
    left_dir = os.path.join(video_dir, "left")
    right_dir = os.path.join(video_dir, "right")
    
    left_video = os.path.join(video_dir, "left.h264")
    right_video = os.path.join(video_dir, "right.h264")
    
    left_timecode = os.path.join(video_dir, "left.txt")
    right_timecode = os.path.join(video_dir, "right.txt")

    if not args.use_existing_images:
        logging.info(f"Interval seconds: {INTERVAL_SECONDS}")
        logging.info("Extracting frames from left video...")
        extract_frames_from_h264_vfr(left_video, left_timecode, left_dir, INTERVAL_SECONDS)
        
        logging.info("Extracting frames from right video...")
        extract_frames_from_h264_vfr(right_video, right_timecode, right_dir, INTERVAL_SECONDS)

    imagesLeft = sorted(glob.glob(os.path.join(left_dir, "*.png")))
    imagesRight = sorted(glob.glob(os.path.join(right_dir, "*.png")))
    
    if not imagesLeft or not imagesRight:
        logging.error("No calibration images found. Expected PNGs in %s and %s.", left_dir, right_dir)
        raise RuntimeError(
            "No calibration images found. Expected PNGs in "
            f"{left_dir} and {right_dir}."
        )

    max_diff = config.get('max_pair_time_diff_seconds', INTERVAL_SECONDS / 2)
    imagesLeft, imagesRight = match_stereo_pairs_by_timestamp(imagesLeft, imagesRight, max_diff_seconds=max_diff)
    if not imagesLeft:
        raise RuntimeError("No stereo pairs could be matched by timestamp.")
    if len(imagesLeft) != len(imagesRight):
        logging.warning(f"Warning: {len(imagesLeft)} left images, {len(imagesRight)} right images.")

    if args.use_stored_calibration:
        calib_file = os.path.join(args.output_directory, "camera_calibration.npz")
        logging.info(f"Loading camera calibration from {calib_file}")
        calib_data = np.load(calib_file)
        cameraMatrixL = calib_data['cameraMatrixL']
        distL = calib_data['distL']
        cameraMatrixR = calib_data['cameraMatrixR']
        distR = calib_data['distR']
        frame_size = tuple(calib_data['frame_size'])
        # You still need objpoints, imgpointsL, imgpointsR for stereo calibration
        objpoints, imgpointsL, imgpointsR, _ = find_chessboard_corners(
            imagesLeft, imagesRight, CHESSBOARD_SIZE, CRITERIA, CHECKERBOARD_SQUARES_SIZE)
    else:
        objpoints, imgpointsL, imgpointsR, frame_size = find_chessboard_corners(
            imagesLeft, imagesRight, CHESSBOARD_SIZE, CRITERIA, CHECKERBOARD_SQUARES_SIZE)
        if frame_size is None or not objpoints:
            logging.error("No valid calibration pairs found. Check chessboard size and images.")
            raise RuntimeError("No valid calibration pairs found. Check chessboard size and images.")
        calib_results = calibrate_cameras(objpoints, imgpointsL, imgpointsR, frame_size)
        cameraMatrixL, distL = calib_results[0], calib_results[1]
        cameraMatrixR, distR = calib_results[3], calib_results[4]
        # Save calibration results
        calib_save_path = os.path.join(args.output_directory, "camera_calibration.npz")
        np.savez(calib_save_path,
            cameraMatrixL=cameraMatrixL,
            distL=distL,
            cameraMatrixR=cameraMatrixR,
            distR=distR,
            frame_size=np.array(frame_size)
        )
        logging.info(f"Camera calibration saved to {calib_save_path}")

    rot, trans, essentialMatrix, fundamentalMatrix, cameraMatrixL, distL, cameraMatrixR, distR = calibrate_stereo_camera(
        objpoints, imgpointsL, imgpointsR, cameraMatrixL, distL, cameraMatrixR, distR, frame_size)

    print("\n--- Stereo Extrinsics ---")
    print("Translation Vector (T):\n", trans)
    print("Rotation Matrix (R):\n", rot)

    stereoMapL, stereoMapR, projMatrixL, projMatrixR, Q, roi_L, roi_R = compute_rectification_maps(
        cameraMatrixL, distL, cameraMatrixR, distR, frame_size, rot, trans)

    print_map_stats(stereoMapL, stereoMapR)

    # Sanity check: rectification maps should cover the image area
    mapL_x_min, mapL_x_max = np.min(stereoMapL[0]), np.max(stereoMapL[0])
    mapL_y_min, mapL_y_max = np.min(stereoMapL[1]), np.max(stereoMapL[1])
    w, h = frame_size
    if mapL_x_max < 0 or mapL_x_min > w or mapL_y_max < 0 or mapL_y_min > h:
        logging.warning("WARNING: Rectification maps are mapping outside the image bounds! "
                       "Calibration may be incorrect.")

    output_xml = os.path.join(args.output_directory, "stereoMap.xml")
    os.makedirs(args.output_directory, exist_ok=True)
    save_stereo_maps(stereoMapL, stereoMapR, output_xml)

    logging.info("Left Camera Matrix:\n%s", cameraMatrixL)
    logging.info("Right Camera Matrix:\n%s", cameraMatrixR)

if __name__ == '__main__':
    main()