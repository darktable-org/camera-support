package main

import (
	"encoding/csv"
	"encoding/json"
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

type camera struct {
	Maker         string
	Model         string
	Aliases       []string
	Formats       []string // rawspeed modes
	WBPresets     bool
	NoiseProfiles bool
	RSSupported   string // rawspeed support
	Decoder       string // rawspeed | libraw
}

type stats struct {
	cameras       int
	aliases       int
	rawspeed      int
	libraw        int
	generic       int
	wbPresets     int
	noiseProfiles int
}

func main() {
	var options struct {
		rawspeedPath      string
		librawPath        string
		wbpresetsPath     string
		noiseprofilesPath string
		stats             bool
		outputFormat      string
		outputFile        string
	}

	flag.StringVar(&options.rawspeedPath, "rawspeed", "data/cameras.xml", "rawspeed cameras.xml location. URL or relative local path")
	flag.StringVar(&options.librawPath, "libraw", "data/libraw.tsv", "libraw.tsv location. URL or relative local path")
	flag.StringVar(&options.wbpresetsPath, "wbpresets", "data/wb_presets.json", "wb_presets.json location. URL or relative local path")
	flag.StringVar(&options.noiseprofilesPath, "noiseprofiles", "data/noiseprofiles.json", "noiseprofiles.json location. URL or relative local path")
	flag.BoolVar(&options.stats, "stats", false, "Print statistics")
	flag.StringVar(&options.outputFormat, "format", "tsv", "Output format")
	flag.StringVar(&options.outputFile, "out", "", "Output file")
	flag.Parse()

	cameras := map[string]camera{}

	loadCamerasXML(cameras, options.rawspeedPath)
	loadLibRawTSV(cameras, options.librawPath)

	loadWBPresets(cameras, options.wbpresetsPath)
	loadNoiseProfiles(cameras, options.noiseprofilesPath)

	stats := generateStats(cameras)

	////  Output  ////

	camerasOrder := make([]string, 0, len(cameras))
	for k := range cameras {
		camerasOrder = append(camerasOrder, k)
	}

	sort.Strings(camerasOrder)

	for _, k := range camerasOrder {
		c := cameras[k]
		fmt.Println(c.Maker, "/ "+c.Model, "/ "+c.Decoder, "/", c.WBPresets, "/", c.NoiseProfiles, "/ "+c.RSSupported+" /", c.Aliases, len(c.Aliases), "/", c.Formats, len(c.Formats), "/", k)
	}

	if options.stats == true {
		fmt.Println("\nCameras:\t", stats.cameras)
		fmt.Println("  rawspeed:\t", stats.rawspeed)
		fmt.Println("  LibRaw:\t", stats.libraw)
		fmt.Println("  Generic:\t", stats.generic)
		fmt.Println("Aliases:\t", stats.aliases)
		fmt.Println("WB Presets:\t", stats.wbPresets)
		fmt.Println("Noise Profiles:\t", stats.noiseProfiles)
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

func loadCamerasXML(cameras map[string]camera, path string) {
	camerasXML := etree.NewDocument()
	if err := camerasXML.ReadFromBytes(getData(path)); err != nil {
		log.Fatal(err)
	}

	root := camerasXML.SelectElement("Cameras")
	for _, c := range root.SelectElements("Camera") {
		maker := ""
		model := ""
		key := ""

		if id := c.SelectElement("ID"); id != nil {
			maker = id.SelectAttrValue("make", "")
			model = id.SelectAttrValue("model", "")
			key = strings.ToLower(maker + " " + model)
		} else {
			maker = c.SelectAttrValue("make", "")
			model = c.SelectAttrValue("model", "")
			key = strings.ToLower(maker + " " + model)

			// fmt.Println("= No ID element")
			// fmt.Println("  "+make, "/ "+model)

			// if model == "" {
			// 	fmt.Println("= No Model in Camera element")
			// 	fmt.Println("  "+make, "/ "+model)
			// }
		}

		camera := cameras[key]
		camera.Maker = maker
		camera.Model = model

		if aliases := c.SelectElement("Aliases"); aliases != nil {
			// fmt.Println("== " + key + " Aliases ==")
			for _, a := range aliases.SelectElements("Alias") {
				alias := ""
				id := a.SelectAttrValue("id", "")
				val := a.Text()
				if id == "" {
					// Sometimes <Alias> doesn't have an id attribute, so use the text instead
					// Not ideal, but probably the best that can be done for now
					// Would be better if cameras.xml was consistent
					alias, _ = strings.CutPrefix(val, maker+" ")
				} else {
					alias = id
				}
				// fmt.Println("  id:\t" + id)
				// fmt.Println("  val:\t" + val)
				// fmt.Println("  alias:\t" + alias)
				camera.Aliases = append(camera.Aliases, alias)
			}
		}

		if format := c.SelectAttrValue("mode", ""); format != "" {
			camera.Formats = append(camera.Formats, format)
		} //  else {
		// 	camera.Formats = append(camera.Formats, "default")
		// }

		camera.RSSupported = c.SelectAttrValue("supported", "")
		if camera.RSSupported == "" {
			camera.Decoder = "rawspeed"
		}

		cameras[key] = camera
	}
}

func loadLibRawTSV(cameras map[string]camera, path string) {
	librawData := getData(path)
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
		key := strings.ToLower(maker + " " + model)

		camera := cameras[key]
		camera.Maker = maker
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

func loadWBPresets(cameras map[string]camera, path string) {
	type Presets struct {
		WBPresets []struct {
			Maker  string `json:"maker"`
			Models []struct {
				Model string `json:"model"`
			} `json:"models"`
		} `json:"wb_presets"`
	}

	jsonBytes := getData(path)

	var presets Presets
	err := json.Unmarshal(jsonBytes, &presets)
	if err != nil {
		fmt.Println("error:", err)
	}

	for _, v := range presets.WBPresets {
		for _, m := range v.Models {
			key := strings.ToLower(v.Maker + " " + m.Model)
			camera := cameras[key]
			if camera.Maker == "" {
				camera.Decoder = "generic"
			}
			camera.Maker = v.Maker
			camera.Model = m.Model
			camera.WBPresets = true
			cameras[key] = camera
		}
	}
}

func loadNoiseProfiles(cameras map[string]camera, path string) {
	type Profiles struct {
		Noiseprofiles []struct {
			Maker  string `json:"maker"`
			Models []struct {
				Model string `json:"model"`
			} `json:"models"`
		} `json:"noiseprofiles"`
	}

	jsonBytes := getData(path)

	var profiles Profiles
	err := json.Unmarshal(jsonBytes, &profiles)
	if err != nil {
		fmt.Println("error:", err)
	}

	for _, v := range profiles.Noiseprofiles {
		for _, m := range v.Models {
			key := strings.ToLower(v.Maker + " " + m.Model)
			camera := cameras[key]
			if camera.Maker == "" {
				camera.Decoder = "generic"
			}
			camera.Maker = v.Maker
			camera.Model = m.Model
			camera.NoiseProfiles = true
			cameras[key] = camera
		}
	}
}

func generateStats(cameras map[string]camera) stats {

	s := stats{
		cameras:       0,
		aliases:       0,
		rawspeed:      0,
		libraw:        0,
		generic:       0,
		wbPresets:     0,
		noiseProfiles: 0,
	}

	for _, c := range cameras {
		if c.Decoder == "" {
			// We only want actually supported cameras
			continue
		} else if c.Decoder == "rawspeed" {
			s.cameras += 1
			s.rawspeed += 1
		} else if c.Decoder == "libraw" {
			s.cameras += 1
			s.libraw += 1
		} else if c.Decoder == "generic" {
			s.cameras += 1
			s.generic += 1
		}

		s.aliases += len(c.Aliases)

		if c.NoiseProfiles == true {
			s.noiseProfiles += 1
		}

		if c.WBPresets == true {
			s.wbPresets += 1
		}
	}

	return s
}
