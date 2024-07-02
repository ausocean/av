#!/bin/bash
# pkg-upgrade.sh - AusOcean package upgrade script.
# Copyright (C) 2024 the Australian Ocean Lab (AusOcean)
# Author: Alan Noble <alan@ausocean.org>
#
# Packages describe the software components used by AusOcean devices.
# This script downloads the package for a device and (optional) version,
# then installs all components that are new or changed.
#
# External dependencies: md5sum, jq
Usage="Usage: pkg-upgrade.sh [-v | -version | device [pkg-version]]"
ScriptVersion="v0.2.0"
LogFile="/var/log/netsender/stream.log"
URL="http://center.cloudblue.org/dl"
#Debug=1

# log wraps echo with the current date and time.
function log() {
  now=$(date '+%Y-%m-%d %H:%M:%S')
  echo "$now: $@"
}

if [ "$1" == "-version" ] || [ "$1" == "-v" ]; then
  echo "$ScriptVersion"
  exit 0
fi

# Redirect output to log file, unless debugging.
if [ -z "$Debug" ]; then
  exec 2> $LogFile
  exec 1>&2
fi

# Get device and (optional) package version.
if [[ -z "$1" ]]; then
  log "Error: missing required argument (device)" >&2
  exit 1
fi
Device="$1"
PkgVersion="@latest"
if [[ -n "$2" ]]; then
  PkgVersion="$2"
fi

log "Info: Commencing upgrade of $Device $PkgVersion"

# Fetch the requested package.
PkgFile="/tmp/pkg.json"
PkgURL="$URL/$Device/pkg/$PkgVersion/pkg.json"
if [ -n "$Debug" ]; then log "Debug: Downloading $PkgURL"; fi
curl -s "$PkgURL" -o "$PkgFile"
if [ $? -ne 0 ]; then
  log "Error: could not get $PkgURL"
  exit 1
fi
Pkg=$(cat "$PkgFile")
if [ $? -ne 0 ]; then
  log "Error: could not read $PkgFile"
  exit 1
fi
NumComponents=$(jq -r '.components | length' <<< "$Pkg")
if [ $? -ne 0 ]; then
  log "Error: invalid package JSON at $PkgURL"
  exit 1
fi
if [ -n "$Debug" ]; then log "Debug: $Device $PkgVersion has $NumComponents components"; fi
if [[ -z $NumComponents ]]; then
  log "Info: $Device $PkgVersion has no components to upgrade"
  exit 0
fi

# Pass 1: Download and check each changed or new component.
Changed=0
TmpDir="/tmp/$Device/$PkgVersion"
mkdir -p "$TmpDir"
for (( i = 0; i < $NumComponents; i++ )); do
  name=$(jq -r ".components[$i] | .name" <<< "$Pkg")
  shared=$(jq -r ".components[$i] | .shared" <<< "$Pkg")
  dir=$(jq -r ".components[$i] | .dir" <<< "$Pkg")
  version=$(jq -r ".components[$i] | .version" <<< "$Pkg")
  expectedChecksum=$(jq -r ".components[$i] | .checksum" <<< "$Pkg")
  checksum=0
  if [ -f "$dir/$name" ]; then
    checksum=$(md5sum "$dir/$name" | cut -d ' ' -f 1)
  else
    log "Warning: $dir/$name does not exist"
  fi
  if [ "$checksum" == "$expectedChecksum" ]; then
    if [ -n "$Debug" ]; then log "Debug: $Device/$name $version unchanged"; fi
  else
    (( Changed++ ))
    log "Info: Upgrading $Device/$name/$version"
    if [ "$shared" == "true" ]; then
      parent="share"
      log "Info: $Device/$name/$version is shared"
    else
      parent="$Device"
    fi
    url="$URL/$parent/$name/$version/$name.gz"
    if [ -n "$Debug" ]; then log "Debug: Downloading $url"; fi
    tmpFile="$TmpDir/$name"
    curl -s -o "$tmpFile.gz" "$url"
    if [ -f "$tmpFile.gz" ]; then
      if [ -n "$Debug" ]; then log "Debug: Successfully downloaded $tmpFile.gz"; fi
      gunzip -f "$tmpFile.gz"
      if [ $? -ne 0 ]; then
        log "Error: failed to unzip $tmpFile.gz"
        exit 1
      else
        if [ -n "$Debug" ]; then log "Debug: Unzipped $tmpFile.gz"; fi
      fi
      checksum=$(md5sum "$tmpFile" | cut -d ' ' -f 1)
      if [ "$checksum" != "$expectedChecksum" ]; then
        log "Error: $tmpFile checksum $checksum does not match $expectedChecksum"
        exit 1
      fi
    else
      log "Error: failed to download $url"
      exit 1
    fi
  fi
done

if [ "$Changed" -eq 0 ]; then
  log "Info: No changed components for $Device $PkgVersion"
  exit 0
fi

# Pass 2: Copy new/changed components to their proper place.
Updated=0
for (( i = 0; i < $NumComponents; i++ )); do
  name=$(jq -r ".components[$i] | .name" <<< "$Pkg")
  dir=$(jq -r ".components[$i] | .dir" <<< "$Pkg")
  executable=$(jq -r ".components[$i] | .executable" <<< "$Pkg")
  cmd=$(jq -r ".components[$i] | .cmd" <<< "$Pkg")
  expectedChecksum=$(jq -r ".components[$i] | .checksum" <<< "$Pkg")
  checksum=0
  if [ -f "$dir/$name" ]; then
    checksum=$(md5sum "$dir/$name" | cut -d ' ' -f 1)
  fi
  if [ "$checksum" != "$expectedChecksum" ]; then
    if [ -f "$dir/$name" ]; then
      # Back up existing file, if any.
      mv -f "$dir/$name" "$dir/$name.bak"
      if [ $? -ne 0 ]; then
        log "Error: could not back up $dir/$name"
        break
      fi
    fi
    # Update the file.
    cp -pf "$TmpDir/$name" "$dir/$name"
    if [ $? -ne 0 ]; then
      log "Error: could not replace $dir/$name"
      break
    fi
    # Set executable bit for executables.
    if [ "$executable" == "true" ]; then
      chmod +x "$dir/$name"
      if [ $? -ne 0 ]; then
        log "Error: could not chmod $dir/$name"
        break
      fi
    fi
    # Run optional command.
    if [ "$cmd" != "null" ]; then
      cmd="${cmd//\$name/$name}"
      cmd="${cmd//\$dir/$dir}"
      if [ -n "$Debug" ]; then log "Debug: Executing '$cmd'"; fi
      bash -c "$cmd" > /dev/null 2>&1
      if [ $? -ne 0 ]; then
        log "Error: could not execute '$cmd'"
        break
      fi
    fi
    (( Updated++ ))
    if [ -n "$Debug" ]; then log "Debug: $dir/$name updated"; fi
  fi
done

if [ "$Updated" != "$Changed" ]; then
  # Pass 3A: Unsuccessul upgrade; restore backups.
  log "Error: only updated $Updated of $Changed components; reverting"
  for (( i = 0; i < $NumComponents; i++ )); do
    name=$(jq -r ".components[$i] | .name" <<< "$Pkg")
    dir=$(jq -r ".components[$i] | .dir" <<< "$Pkg")
    if [ -f "$dir/$name.bak" ]; then
      if [ -n "$Debug" ]; then log "Debug: Restoring $dir/$name"; fi
      mv -f "$dir/$name.bak" "$dir/$name"
      if [ $? -ne 0 ]; then
        log "Error: could not restore $dir/$name"
      fi
    fi
  done
  exit 1
fi

# Pass 3B: Successful upgrade; remove backups.
if [ -n "$Debug" ]; then log "Debug: Removing backups"; fi
for (( i = 0; i < $NumComponents; i++ )); do
  name=$(jq -r ".components[$i] | .name" <<< "$Pkg")
  dir=$(jq -r ".components[$i] | .dir" <<< "$Pkg")
  if [ -f "$dir/$name.bak" ]; then
    rm -f "$dir/$name.bak"
    if [ -n "$Debug" ]; then log "Debug: removed $dir/$name.bak"; fi
    if [ $? -ne 0 ]; then
      log "Error: could not remove $dir/$name.bak"
    fi
  fi
done
if [ -z "$Debug" ]; then rm -rf "$TmpDir"; fi

# Save a copy of the package.
DataDir="/opt/ausocean/data"
mkdir -p "$DataDir"
mv -f "$PkgFile" "$DataDir"
if [ $? -ne 0 ]; then
  log "Warning: could not move $PkgFile"
fi

log "Info: Updated $Updated components for $Device $PkgVersion"
exit 0
