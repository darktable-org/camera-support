package main

import (
	"flag"
	"fmt"
)

// https://www.darktable.org/resources/camera-support/
//
// https://github.com/darktable-org/rawspeed/blob/develop/data/cameras.xml
// https://github.com/darktable-org/darktable/blob/master/src/imageio/imageio_libraw.c
//
// https://github.com/darktable-org/darktable/blob/master/data/noiseprofiles.json
// https://github.com/darktable-org/darktable/blob/master/data/wb_presets.json

/*
   []cameras
       camera struct {
           Make string
           Model string
           Aliases []string
           Modes []string
           WBPresets bool
           NoiseProfiles bool
           RSSupported string // From rawspeed
           Decoder string // rawspeed | libraw
       }
*/

func main() {
	var (
		rawspeedPath      string
		librawPath        string
		wbpresetsPath     string
		noiseprofilesPath string
		outputFormat      string
		outputFile        string
	)

	flag.StringVar(&rawspeedPath, "rawspeed", "", "rawspeed cameras.xml location. URL or relative local path")
	flag.StringVar(&librawPath, "libraw", "", "LibRaw camera list location")
	flag.StringVar(&wbpresetsPath, "wbpresets", "", "wb_presets.json location")
	flag.StringVar(&noiseprofilesPath, "noiseprofiles", "", "noiseprofiles.json location")
	flag.StringVar(&outputFormat, "format", "", "Output format")
	flag.StringVar(&outputFile, "out", "", "Output file")
	flag.Parse()

	fmt.Println(rawspeedPath)
	fmt.Println(librawPath)
	fmt.Println(wbpresetsPath)
	fmt.Println(noiseprofilesPath)
	fmt.Println(outputFormat)
	fmt.Println(outputFile)
}
