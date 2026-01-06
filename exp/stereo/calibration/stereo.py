import cv2
import glob
import os 

def undistortRectify(frameR, frameL):
    # Undistort and rectify images
    undistortedL= cv2.remap(frameL, stereoMapL_x, stereoMapL_y, cv2.INTER_LANCZOS4, cv2.BORDER_CONSTANT, 0)
    undistortedR= cv2.remap(frameR, stereoMapR_x, stereoMapR_y, cv2.INTER_LANCZOS4, cv2.BORDER_CONSTANT, 0)

    return undistortedR, undistortedL

root_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", "..", ".."))
calib_root = os.path.join(root_dir, "calibrationFiles")
left_dir = os.path.join(calib_root, "left")
right_dir = os.path.join(calib_root, "right")
imagesLeft = sorted(glob.glob(os.path.join(left_dir, "*.png"))) # Path to left camera images
imagesRight = sorted(glob.glob(os.path.join(right_dir, "*.png"))) # Path to right camera images


cv_file = cv2.FileStorage()
cv_file.open('stereoMap.xml', cv2.FileStorage_READ)
stereoMapL_x = cv_file.getNode('stereoMapL_x').mat()
stereoMapL_y = cv_file.getNode('stereoMapL_y').mat()
stereoMapR_x = cv_file.getNode('stereoMapR_x').mat()
stereoMapR_y = cv_file.getNode('stereoMapR_y').mat()

for imgLeft, imgRight in zip(imagesLeft, imagesRight): # Zip images together

    undistoredR, undistortedL = undistortRectify(imgRight, imgLeft)
    cv2.imshow("Undistorted Left", undistortedL)
    cv2.imshow("Undistorted Right", undistoredR)
