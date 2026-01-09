import glob
import os
import cv2 as cv
import numpy as np

chessboardSize = (10,7) # Use (x-1, y-1) to the actual size of chessboad pattern
frameSize = None # Resolution of both cameras.

criteria = (cv.TERM_CRITERIA_EPS + cv.TERM_CRITERIA_MAX_ITER, 30, 0.001) # Stop criteria for cornerSubPix

objp = np.zeros((chessboardSize[0] * chessboardSize[1], 3), np.float32)
objp[:,:2] = np.mgrid[0:chessboardSize[0],0:chessboardSize[1]].T.reshape(-1,2)

objpoints = [] # 3d point in real world space
imgpointsL = [] # 2d points in image plane.
imgpointsR = [] # 2d points in image plane.

root_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", "..", ".."))
calib_root = os.path.join(root_dir, "calibrationFiles")
left_dir = os.path.join(calib_root, "left")
right_dir = os.path.join(calib_root, "right")
imagesLeft = sorted(glob.glob(os.path.join(left_dir, "*.png"))) # Path to left camera images
imagesRight = sorted(glob.glob(os.path.join(right_dir, "*.png"))) # Path to right camera images

if not imagesLeft or not imagesRight:
    raise RuntimeError(
        "No calibration images found. Expected PNGs in "
        f"{left_dir} and {right_dir}."
    )

if len(imagesLeft) != len(imagesRight):
    print(f"Warning: {len(imagesLeft)} left images, {len(imagesRight)} right images.")

for imgLeft, imgRight in zip(imagesLeft, imagesRight): # Zip images together

    imgL = cv.imread(imgLeft)
    imgR = cv.imread(imgRight)
    if imgL is None or imgR is None:
        raise RuntimeError(f"Failed to read calibration pair: {imgLeft}, {imgRight}")
    grayL = cv.cvtColor(imgL, cv.COLOR_BGR2GRAY) #greyscale
    grayR = cv.cvtColor(imgR, cv.COLOR_BGR2GRAY) #greyscale
    if grayL.shape != grayR.shape:
        raise RuntimeError("Left/right calibration images must have the same resolution.")
    if frameSize is None:
        frameSize = grayL.shape[::-1]
    elif frameSize != grayL.shape[::-1]:
        raise RuntimeError("Calibration images must all have the same resolution.")

    # Find the chess board corners
    retL, cornersL = cv.findChessboardCorners(grayL, chessboardSize, None)
    retR, cornersR = cv.findChessboardCorners(grayR, chessboardSize, None)
    
    # If found, add object points, image points (after refining them)
    if retL and retR:

        objpoints.append(objp)

        cornersL = cv.cornerSubPix(grayL, cornersL, (11,11), (-1,-1), criteria)
        imgpointsL.append(cornersL)

        cornersR = cv.cornerSubPix(grayR, cornersR, (11,11), (-1,-1), criteria)
        imgpointsR.append(cornersR)

        # Draw and display the corners
        cv.drawChessboardCorners(imgL, chessboardSize, cornersL, retL)
        cv.imshow('img left', imgL)
        cv.drawChessboardCorners(imgR, chessboardSize, cornersR, retR)
        cv.imshow('img right', imgR)
        cv.waitKey(1000)


cv.destroyAllWindows()

# Calibration for left and right camera individually

if frameSize is None or not objpoints:
    raise RuntimeError("No valid calibration pairs found. Check chessboard size and images.")

retL, cameraMatrixL, distL, rvecsL, tvecsL = cv.calibrateCamera(objpoints, imgpointsL, frameSize, None, None)
heightL, widthL, channelsL = imgL.shape
newCameraMatrixL, roi_L = cv.getOptimalNewCameraMatrix(cameraMatrixL, distL, (widthL, heightL), 1, (widthL, heightL))

retR, cameraMatrixR, distR, rvecsR, tvecsR = cv.calibrateCamera(objpoints, imgpointsR, frameSize, None, None)
heightR, widthR, channelsR = imgR.shape
newCameraMatrixR, roi_R = cv.getOptimalNewCameraMatrix(cameraMatrixR, distR, (widthR, heightR), 1, (widthR, heightR))

# Calibration for stereo camera
flags = 0
flags |= cv.CALIB_FIX_INTRINSIC
# Here we fix the intrinsic camara matrixes so that only Rot, Trns, Emat and Fmat are calculated.
# Hence intrinsic parameters are the same 

criteria_stereo= (cv.TERM_CRITERIA_EPS + cv.TERM_CRITERIA_MAX_ITER, 30, 0.001) #stop calibration when 30 iterations and no change (0.001)

# This step is performed to transformation between the two cameras and calculate Essential and Fundamental matrix
retStereo, newCameraMatrixL, distL, newCameraMatrixR, distR, rot, trans, essentialMatrix, fundamentalMatrix = cv.stereoCalibrate(objpoints, imgpointsL, imgpointsR, newCameraMatrixL, distL, newCameraMatrixR, distR, frameSize, criteria_stereo, flags)

print(newCameraMatrixL)
print(newCameraMatrixR)

#rectification map 

rectifyScale= 1
rectL, rectR, projMatrixL, projMatrixR, Q, roi_L, roi_R= cv.stereoRectify(newCameraMatrixL, distL, newCameraMatrixR, distR, frameSize, rot, trans, rectifyScale,(0,0))

stereoMapL = cv.initUndistortRectifyMap(newCameraMatrixL, distL, rectL, projMatrixL, frameSize, cv.CV_16SC2)
stereoMapR = cv.initUndistortRectifyMap(newCameraMatrixR, distR, rectR, projMatrixR, frameSize, cv.CV_16SC2)

#save recitified map
print("Saving parameters!")
cv_file = cv.FileStorage('stereoMap.xml', cv.FILE_STORAGE_WRITE)

cv_file.write('stereoMapL_x',stereoMapL[0])
cv_file.write('stereoMapL_y',stereoMapL[1])
cv_file.write('stereoMapR_x',stereoMapR[0])
cv_file.write('stereoMapR_y',stereoMapR[1])

cv_file.release()

# Camera parameters to undistort and rectify images
