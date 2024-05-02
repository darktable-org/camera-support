package main

import (
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/beevik/etree"
)

// https://www.darktable.org/resources/camera-support/
//
// https://github.com/darktable-org/rawspeed/blob/develop/data/cameras.xml
// https://github.com/darktable-org/darktable/blob/master/src/imageio/imageio_libraw.c
//
// https://github.com/darktable-org/darktable/blob/master/data/noiseprofiles.json
// https://github.com/darktable-org/darktable/blob/master/data/wb_presets.json

type camera struct {
	Make          string
	Model         string
	Aliases       []string
	Formats       []string // rawspeed modes
	WBPresets     bool
	NoiseProfiles bool
	RSSupported   string // rawspeed support
	Decoder       string // rawspeed | libraw
}

func main() {
	var options struct {
		rawspeedPath      string
		librawPath        string
		wbpresetsPath     string
		noiseprofilesPath string
		outputFormat      string
		outputFile        string
	}

	flag.StringVar(&options.rawspeedPath, "rawspeed", "data/cameras.xml", "rawspeed cameras.xml location. URL or relative local path")
	flag.StringVar(&options.librawPath, "libraw", "data/libraw.tsv", "libraw.tsv location. URL or relative local path")
	flag.StringVar(&options.wbpresetsPath, "wbpresets", "data/wb_presets.json", "wb_presets.json location. URL or relative local path")
	flag.StringVar(&options.noiseprofilesPath, "noiseprofiles", "data/noiseprofiles.json", "noiseprofiles.json location. URL or relative local path")
	flag.StringVar(&options.outputFormat, "format", "tsv", "Output format")
	flag.StringVar(&options.outputFile, "out", "", "Output file")
	flag.Parse()

	// fmt.Println(options.rawspeedPath)
	// fmt.Println(options.librawPath)
	// fmt.Println(options.wbpresetsPath)
	// fmt.Println(options.noiseprofilesPath)
	// fmt.Println(options.outputFormat)
	// fmt.Println(options.outputFile)

	cameras := map[string]camera{}

	loadCamerasXML(cameras, options.rawspeedPath)

	////  Output  ////

	camerasOrder := make([]string, 0, len(cameras))
	for k := range cameras {
		camerasOrder = append(camerasOrder, k)
	}

	sort.Strings(camerasOrder)

	for _, k := range camerasOrder {
		c := cameras[k]
		fmt.Println(c.Make, "/ "+c.Model, "/ "+c.Decoder, "/", c.WBPresets, "/", c.NoiseProfiles, "/ "+c.RSSupported+" /", c.Aliases, len(c.Aliases), "/", c.Formats, len(c.Formats))
	}
}

func loadCamerasXML(cameras map[string]camera, camerasXMLPath string) {
	camerasXML := etree.NewDocument()
	if err := camerasXML.ReadFromFile(camerasXMLPath); err != nil {
		panic(err)
	}

	root := camerasXML.SelectElement("Cameras")
	for _, c := range root.SelectElements("Camera") {
		make := ""
		model := ""
		key := ""

		if id := c.SelectElement("ID"); id != nil {
			make = id.SelectAttrValue("make", "")
			model = id.SelectAttrValue("model", "")
			key = make + " " + model
		} else {
			make = c.SelectAttrValue("make", "")
			model = c.SelectAttrValue("model", "")
			key = make + " " + model

			// fmt.Println("= No ID element")
			// fmt.Println("  "+make, "/ "+model)

			// if model == "" {
			// 	fmt.Println("= No Model in Camera element")
			// 	fmt.Println("  "+make, "/ "+model)
			// }
		}

		camera := cameras[key]
		camera.Make = make
		camera.Model = model

		if aliases := c.SelectElement("Aliases"); aliases != nil {
			// fmt.Println("== " + key + " Aliases ==")
			for _, a := range aliases.SelectElements("Alias") {
				alias := ""
				id := a.SelectAttrValue("id", "")
				val := a.Text()
				if id == "" {
					// Not ideal, but probably the best that can be done for now
					alias, _ = strings.CutPrefix(val, make+" ")
				} else {
					alias = id
				}
				// fmt.Println("  id:    \t" + id)
				// fmt.Println("  val:   \t" + val)
				// fmt.Println("  alias: \t" + alias)
				camera.Aliases = append(camera.Aliases, alias)
			}
		}

		if format := c.SelectAttrValue("mode", ""); format != "" {
			camera.Formats = append(camera.Formats, format)
		} else {
			camera.Formats = append(camera.Formats, "default")
		}

		camera.RSSupported = c.SelectAttrValue("supported", "")
		if camera.RSSupported == "" {
			camera.Decoder = "rawspeed"
		}

		cameras[key] = camera
	}
}
