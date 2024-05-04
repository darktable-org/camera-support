package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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
	loadLibRawTSV(cameras, options.librawPath)

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

func getData(path string) []byte {
	if strings.HasPrefix(path, "https://") {
		// fmt.Println("-- Getting data from URL")
		res, err := http.Get(path)
		if err != nil {
			log.Fatal(err)
		}
		data, err := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode > 299 {
			log.Fatalf("Response failed with status code: %d and\nbody: %s\n", res.StatusCode, data)
		}
		if err != nil {
			log.Fatal(err)
		}
		return data
	} else {
		// fmt.Println("-- Getting data from local file")
		data, err := os.ReadFile(path)
		if err != nil {
			log.Fatal(err)
		}
		return data
	}
}

func loadCamerasXML(cameras map[string]camera, rawspeedPath string) {
	camerasXML := etree.NewDocument()
	if err := camerasXML.ReadFromBytes(getData(rawspeedPath)); err != nil {
		panic(err)
	}

	root := camerasXML.SelectElement("Cameras")
	for _, c := range root.SelectElements("Camera") {
		maker := ""
		model := ""
		key := ""

		if id := c.SelectElement("ID"); id != nil {
			maker = id.SelectAttrValue("make", "")
			model = id.SelectAttrValue("model", "")
			key = maker + " " + model
		} else {
			maker = c.SelectAttrValue("make", "")
			model = c.SelectAttrValue("model", "")
			key = maker + " " + model

			// fmt.Println("= No ID element")
			// fmt.Println("  "+make, "/ "+model)

			// if model == "" {
			// 	fmt.Println("= No Model in Camera element")
			// 	fmt.Println("  "+make, "/ "+model)
			// }
		}

		camera := cameras[key]
		camera.Make = maker
		camera.Model = model

		if aliases := c.SelectElement("Aliases"); aliases != nil {
			// fmt.Println("== " + key + " Aliases ==")
			for _, a := range aliases.SelectElements("Alias") {
				alias := ""
				id := a.SelectAttrValue("id", "")
				val := a.Text()
				if id == "" {
					// Not ideal, but probably the best that can be done for now
					alias, _ = strings.CutPrefix(val, maker+" ")
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

func loadLibRawTSV(cameras map[string]camera, librawPath string) {
	librawData := getData(librawPath)
	librawTSV := strings.NewReader(string(librawData))

	reader := csv.NewReader(librawTSV)
	reader.Comma = '\t'
	reader.Read() // use Read to remove the first line
	rows, err := reader.ReadAll()
	if err != nil {
		log.Println("Cannot read libraw.tsv:", err)
	}

	for _, c := range rows {

		maker := c[0]
		model := c[1]
		aliases := c[2]
		formats := c[3]
		key := maker + " " + model

		camera := cameras[key]
		camera.Make = maker
		camera.Model = model

		if aliases != "" {
			// Use a set to ensure no duplicate aliases
			set := make(map[string]struct{})
			if len(camera.Aliases) >= 1 {
				for _, a := range camera.Aliases {
					set[a] = struct{}{}
				}
			}

			for _, a := range strings.Split(aliases, ";") {
				a := strings.Trim(a, " ")
				set[a] = struct{}{}
			}

			camera.Aliases = nil
			for k := range set {
				camera.Aliases = append(camera.Aliases, k)
			}
		}

		if formats != "" {
			// Use a set to ensure no duplicate formats
			set := make(map[string]struct{})
			if len(camera.Formats) >= 1 {
				for _, f := range camera.Formats {
					set[f] = struct{}{}
				}
			}

			for _, f := range strings.Split(formats, ";") {
				f := strings.Trim(f, " ")
				set[f] = struct{}{}
			}

			camera.Formats = nil
			for k := range set {
				camera.Formats = append(camera.Formats, k)
			}
		}

		camera.Decoder = "libraw"

		cameras[key] = camera
	}
}

func loadWBPresets(cameras map[string]camera, wbPresetsPath string) {

}
