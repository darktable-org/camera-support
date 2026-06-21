#!/bin/sh

sourcedir="$1"

go run camera-support.go \
   -libraw="${sourcedir}/src/imageio/imageio_libraw.c" \
   -rawspeed="${sourcedir}/src/external/rawspeed/data/cameras.xml" \
   -wbpresets="${sourcedir}/data/wb_presets.json" \
   -noiseprofiles="${sourcedir}/data/noiseprofiles.json" \
   -format="md" \
   -segments="3" \
   -fields="no-maker" \
   -bools="✅ Yes;❌ No"

