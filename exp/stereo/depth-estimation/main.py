import argparse
import os
import sys
import cv2 as cv
import numpy as np

DEFAULT_LEFT_VIDEO = (
    r"C:\Users\jonty\OneDrive - Flinders\Desktop\AusOcean\Stereo\testVideo\testL.mp4"
)
DEFAULT_RIGHT_VIDEO = (
    r"C:\Users\jonty\OneDrive - Flinders\Desktop\AusOcean\Stereo\testVideo\testR.mp4"
)
DEFAULT_MAP_PATH = os.path.abspath(
    os.path.join(os.path.dirname(__file__), "..", "calibration", "stereoMap.xml")
)

def load_stereo_maps(path):
    print(f"Loading stereo maps from: {path}")
    if not os.path.exists(path):
        raise FileNotFoundError(f"stereoMap.xml not found: {path}")

    fs = cv.FileStorage(path, cv.FILE_STORAGE_READ)
    if not fs.isOpened():
        raise RuntimeError(f"Failed to open stereo map file: {path}")

    map_l_x = fs.getNode("stereoMapL_x").mat()
    map_l_y = fs.getNode("stereoMapL_y").mat()
    map_r_x = fs.getNode("stereoMapR_x").mat()
    map_r_y = fs.getNode("stereoMapR_y").mat()
    fs.release()

    if (
        map_l_x is None
        or map_l_y is None
        or map_r_x is None
        or map_r_y is None
        or map_l_x.size == 0
        or map_l_y.size == 0
        or map_r_x.size == 0
        or map_r_y.size == 0
    ):
        raise RuntimeError("stereoMap.xml missing rectification maps")

    return map_l_x, map_l_y, map_r_x, map_r_y


def build_stereo_matcher(width):
    num_disp = 16 * max(4, ((width // 8) + 15) // 16)
    block_size = 7
    return cv.StereoSGBM_create(
        minDisparity=0,
        numDisparities=num_disp,
        blockSize=block_size,
        P1=8 * 1 * block_size * block_size,
        P2=32 * 1 * block_size * block_size,
        disp12MaxDiff=1,
        uniquenessRatio=10,
        speckleWindowSize=50,
        speckleRange=2,
        preFilterCap=63,
        mode=cv.STEREO_SGBM_MODE_SGBM_3WAY,
    )


def rectify_frames(frame_l, frame_r, maps):
    map_l_x, map_l_y, map_r_x, map_r_y = maps
    rect_l = cv.remap(frame_l, map_l_x, map_l_y, cv.INTER_LANCZOS4)
    rect_r = cv.remap(frame_r, map_r_x, map_r_y, cv.INTER_LANCZOS4)
    return rect_l, rect_r


def parse_args():
    parser = argparse.ArgumentParser(
        description="Compute disparity from stereo videos using stereoMap.xml"
    )
    parser.add_argument("--left", default=DEFAULT_LEFT_VIDEO, help="Path to left video")
    parser.add_argument(
        "--right", default=DEFAULT_RIGHT_VIDEO, help="Path to right video"
    )
    parser.add_argument(
        "--map", dest="map_path", default=DEFAULT_MAP_PATH, help="Path to stereoMap.xml"
    )
    return parser.parse_args()


def main():
    args = parse_args()
    left_path = args.left
    right_path = args.right

    if not os.path.exists(left_path):
        print(f"Left video not found: {left_path}")
        return 1
    if not os.path.exists(right_path):
        print(f"Right video not found: {right_path}")
        return 1

    try:
        maps = load_stereo_maps(args.map_path)
    except (FileNotFoundError, RuntimeError) as exc:
        print(str(exc))
        return 1
    cap_l = cv.VideoCapture(left_path)
    cap_r = cv.VideoCapture(right_path)
    if not cap_l.isOpened() or not cap_r.isOpened():
        print("Failed to open one or both video files.")
        return 1

    ret_l, frame_l = cap_l.read()
    ret_r, frame_r = cap_r.read()
    if not ret_l or not ret_r:
        print("Failed to read the first frame from one or both videos.")
        return 1

    print("map size:", maps[0].shape, "frame size:", frame_l.shape)
    print(
        "map dtype:",
        maps[0].dtype,
        maps[1].dtype,
        maps[2].dtype,
        maps[3].dtype,
    )
    print("frame mean L/R:", f"{frame_l.mean():.2f}", f"{frame_r.mean():.2f}")
    if maps[0].dtype == np.int16 and maps[0].ndim == 3:
        map1, _ = cv.convertMaps(maps[0], maps[1], cv.CV_32FC1)
        x_map = map1[..., 0]
        y_map = map1[..., 1]
    else:
        x_map = maps[0][..., 0] if maps[0].ndim == 3 else maps[0]
        y_map = maps[0][..., 1] if maps[0].ndim == 3 else maps[1]
    oob = (
        (x_map < 0)
        | (x_map >= frame_l.shape[1])
        | (y_map < 0)
        | (y_map >= frame_l.shape[0])
    ).sum()
    print(
        "map1 x range:",
        int(x_map.min()),
        int(x_map.max()),
        "y range:",
        int(y_map.min()),
        int(y_map.max()),
    )
    print(f"map1 out-of-bounds: {oob}/{x_map.size} ({(oob / x_map.size) * 100:.2f}%)")

    if frame_l.shape[:2] != frame_r.shape[:2]:
        print("Left/right frame sizes do not match.")
        return 1

    map_h, map_w = maps[0].shape[:2]
    if (map_h, map_w) != frame_l.shape[:2]:
        print(
            "stereoMap.xml resolution does not match the video resolution. "
            "Recalibrate or use matching videos."
        )
        return 1

    rect_l, rect_r = rectify_frames(frame_l, frame_r, maps)
    rect_pair = np.hstack((rect_l, rect_r))
    cv.imshow("rectified", rect_pair)
    cv.waitKey(0)
    cv.destroyAllWindows()

    matcher = build_stereo_matcher(frame_l.shape[1])

    while True:
        if not ret_l or not ret_r:
            break

        rect_l, rect_r = rectify_frames(frame_l, frame_r, maps)
        gray_l = cv.cvtColor(rect_l, cv.COLOR_BGR2GRAY)
        gray_r = cv.cvtColor(rect_r, cv.COLOR_BGR2GRAY)

        disp = matcher.compute(gray_l, gray_r).astype(np.float32) / 16.0
        disp_vis = cv.normalize(disp, None, 0, 255, cv.NORM_MINMAX)
        disp_vis = disp_vis.astype(np.uint8)

        cv.imshow("rectified left", rect_l)
        cv.imshow("rectified right", rect_r)
        cv.imshow("disparity", disp_vis)

        if cv.waitKey(1) & 0xFF == ord("q"):
            break

        ret_l, frame_l = cap_l.read()
        ret_r, frame_r = cap_r.read()

    cap_l.release()
    cap_r.release()
    cv.destroyAllWindows()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
