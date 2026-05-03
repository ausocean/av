import argparse
import glob
import logging
import os
import re

import cv2 as cv
import numpy as np


def extract_timestamp_from_filename(filepath):
    """Extract timestamp in seconds from a filename like frame_NNNNNN_TTTT.TTTs.png."""
    match = re.search(r'_(\d+(?:\.\d+)?)s\.png$', os.path.basename(filepath))
    return float(match.group(1)) if match else None


def match_stereo_pairs_by_timestamp(images_left, images_right, max_diff_seconds=2.0):
    """Match left and right image paths by closest timestamp within tolerance."""
    left_ts = [(extract_timestamp_from_filename(p), p) for p in images_left]
    right_ts = [(extract_timestamp_from_filename(p), p) for p in images_right]

    if any(t is None for t, _ in left_ts) or any(t is None for t, _ in right_ts):
        logging.warning("Could not parse timestamps; falling back to index-based pairing.")
        return list(images_left), list(images_right)

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
        else:
            logging.debug(f"No right match within {max_diff_seconds}s for {os.path.basename(left_path)}")

    logging.info(f"Matched {len(matched_left)} stereo pairs by timestamp.")
    return matched_left, matched_right


def load_stereo_maps(stereo_map_path):
    """Load rectification maps from a stereoMap.xml file."""
    logging.info(f"Loading stereo maps from {stereo_map_path} ...")
    cv_file = cv.FileStorage(stereo_map_path, cv.FILE_STORAGE_READ)
    if not cv_file.isOpened():
        raise RuntimeError(f"Could not open stereo map file: {stereo_map_path}")
    maps = {
        'L_x': cv_file.getNode('stereoMapL_x').mat(),
        'L_y': cv_file.getNode('stereoMapL_y').mat(),
        'R_x': cv_file.getNode('stereoMapR_x').mat(),
        'R_y': cv_file.getNode('stereoMapR_y').mat(),
    }
    cv_file.release()
    for key, mat in maps.items():
        if mat is None or mat.size == 0:
            raise RuntimeError(f"Stereo map '{key}' is missing or empty in {stereo_map_path}")
        logging.debug(f"  stereoMap{key}: shape={mat.shape}, dtype={mat.dtype}")
    logging.info("Stereo maps loaded.")
    return maps


def undistort_rectify(frame_l, frame_r, maps):
    """Apply rectification maps to a stereo pair."""
    rect_l = cv.remap(frame_l, maps['L_x'], maps['L_y'], cv.INTER_LANCZOS4, borderMode=cv.BORDER_CONSTANT)
    rect_r = cv.remap(frame_r, maps['R_x'], maps['R_y'], cv.INTER_LANCZOS4, borderMode=cv.BORDER_CONSTANT)
    return rect_l, rect_r


def draw_epipolar_lines(img_l, img_r, num_lines=20):
    """Draw horizontal epipolar lines on a side-by-side image for visual verification."""
    combined = np.hstack((img_l, img_r))
    h = combined.shape[0]
    step = h // (num_lines + 1)
    for i in range(1, num_lines + 1):
        y = i * step
        cv.line(combined, (0, y), (combined.shape[1], y), (0, 255, 0), 1)
    return combined


def main():
    logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

    parser = argparse.ArgumentParser(description="Rectify stereo images using calibration maps.")
    parser.add_argument('--stereo_map', type=str, required=True, help='Path to stereoMap.xml file')
    parser.add_argument('--image_dir', type=str, required=True, help='Path to directory containing left/ and right/ subdirectories')
    parser.add_argument('--output_dir', type=str, default=None, help='Directory to save rectified images (optional)')
    parser.add_argument('--show', action='store_true', help='Display rectified images interactively')
    parser.add_argument('--epipolar', action='store_true', help='Overlay epipolar lines on the side-by-side view')
    parser.add_argument('--max_pair_diff', type=float, default=2.0, help='Max timestamp difference (s) for pairing frames')
    args = parser.parse_args()

    left_dir = os.path.join(args.image_dir, "left")
    right_dir = os.path.join(args.image_dir, "right")

    images_left = sorted(glob.glob(os.path.join(left_dir, "*.png")))
    images_right = sorted(glob.glob(os.path.join(right_dir, "*.png")))
    logging.info(f"Found {len(images_left)} left images, {len(images_right)} right images.")

    if not images_left or not images_right:
        raise RuntimeError(f"No PNG images found in {left_dir} or {right_dir}")

    images_left, images_right = match_stereo_pairs_by_timestamp(images_left, images_right, args.max_pair_diff)

    maps = load_stereo_maps(args.stereo_map)

    if args.output_dir:
        out_left = os.path.join(args.output_dir, "left")
        out_right = os.path.join(args.output_dir, "right")
        os.makedirs(out_left, exist_ok=True)
        os.makedirs(out_right, exist_ok=True)
        logging.info(f"Saving rectified images to {args.output_dir}")

    for i, (path_l, path_r) in enumerate(zip(images_left, images_right)):
        frame_l = cv.imread(path_l)
        frame_r = cv.imread(path_r)
        if frame_l is None or frame_r is None:
            logging.warning(f"Could not read pair {i}: {path_l}, {path_r} — skipping.")
            continue

        logging.info(f"[{i+1}/{len(images_left)}] {os.path.basename(path_l)} <-> {os.path.basename(path_r)}")

        rect_l, rect_r = undistort_rectify(frame_l, frame_r, maps)

        if args.output_dir:
            cv.imwrite(os.path.join(out_left, os.path.basename(path_l)), rect_l)
            cv.imwrite(os.path.join(out_right, os.path.basename(path_r)), rect_r)

        if args.show:
            view = draw_epipolar_lines(rect_l, rect_r) if args.epipolar else np.hstack((rect_l, rect_r))
            cv.imshow("Rectified — Left | Right  (any key: next, q: quit)", view)
            key = cv.waitKey(0) & 0xFF
            if key == ord('q'):
                logging.info("Quit by user.")
                break

    cv.destroyAllWindows()
    logging.info("Done.")


if __name__ == '__main__':
    main()
