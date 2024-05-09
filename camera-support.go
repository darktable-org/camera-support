package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
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
	Decoder       string // rawspeed / libraw / unknown
}

type stats struct {
	cameras       int
	aliases       int
	rawspeed      int
	libraw        int
	unknown       int
	unsupported   int
	wbPresets     int
	noiseProfiles int
}

func main() {
	var options struct {
		rawspeedPath      string
		librawPath        string
		wbpresetsPath     string
		noiseprofilesPath string
		stats             string
		format            string
		headers           string
		fields            string
		unsupported       bool
		output            string
	}

	flag.StringVar(&options.rawspeedPath, "rawspeed", "https://raw.githubusercontent.com/darktable-org/rawspeed/develop/data/cameras.xml", "'cameras.xml' location.")
	flag.StringVar(&options.librawPath, "libraw", "https://raw.githubusercontent.com/darktable-org/darktable/master/src/imageio/imageio_libraw.c", "'imageio_libraw.c' location. If empty, LibRaw cameras will not be included.")
	flag.StringVar(&options.wbpresetsPath, "wbpresets", "https://raw.githubusercontent.com/darktable-org/darktable/master/data/wb_presets.json", "'wb_presets.json' location.")
	flag.StringVar(&options.noiseprofilesPath, "noiseprofiles", "https://raw.githubusercontent.com/darktable-org/darktable/master/data/noiseprofiles.json", "'noiseprofiles.json' location.")
	flag.StringVar(&options.stats, "stats", "stdout", "Print statistics. <stdout|table|all|none>")
	flag.StringVar(&options.format, "format", "md", "Output format. <md|html|tsv|none>")
	flag.StringVar(&options.headers, "headers", "", "Segments tables by maker, adding a header using the specified level. <h1-h6>")
	flag.StringVar(&options.fields, "fields", "", "Comma delimited list of fields to print. Default is all. See the 'camera' struct in 'camera-support.go' for valid fields.")
	flag.BoolVar(&options.unsupported, "unsupported", false, "Include unsupported cameras. Also affects statistics.")
	flag.Parse()

	if flag.Arg(0) != "" {
		options.output = flag.Arg(0)
	} else {
		options.output = "stdout"
	}

	cameras := map[string]camera{}

	loadRawspeed(cameras, options.rawspeedPath)

	if options.librawPath != "" {
		loadLibRaw(cameras, options.librawPath)
	}

	loadWBPresets(cameras, options.wbpresetsPath)
	loadNoiseProfiles(cameras, options.noiseprofilesPath)

	stats := generateStats(cameras, options.unsupported)

	////  Output  ////

	// Maps can't be sorted, so create a separate sorted slice for the printing order
	camerasOrder := make([]string, 0, len(cameras))
	for k := range cameras {
		camerasOrder = append(camerasOrder, k)
	}
	sort.Strings(camerasOrder)

	if options.format == "md" {
		_ = generateMD(cameras, camerasOrder, options.unsupported)
	} else if options.format == "html" {
		_ = generateHTML(cameras, camerasOrder, options.unsupported)
	} else if options.format == "tsv" {
		_ = generateTSV(cameras, camerasOrder, options.unsupported)
	} else if options.format == "debug" {
		for _, k := range camerasOrder {
			c := cameras[k]
			fmt.Println(c.Maker, "/ "+c.Model, "/ "+c.Decoder, "/", c.WBPresets, "/", c.NoiseProfiles, "/ "+c.RSSupported+" /", c.Aliases, len(c.Aliases), "/", c.Formats, len(c.Formats), "/", k)
		}
	}

	if options.stats == "stdout" || options.stats == "" {
		if options.output == "stdout" && options.format != "none" {
			fmt.Println("\r")
		}
		fmt.Println("Cameras:\t", stats.cameras)
		fmt.Println("  rawspeed:\t", stats.rawspeed)
		fmt.Println("  LibRaw:\t", stats.libraw)
		fmt.Println("  Unknown:\t", stats.unknown)
		fmt.Println("  Unsupported:\t", stats.unsupported)
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

func loadRawspeed(cameras map[string]camera, path string) {
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

func loadLibRaw(cameras map[string]camera, path string) {
	inStruct := false
	maker := ""
	model := ""
	alias := ""

	librawData := string(getData(path))

	scanner := bufio.NewScanner(strings.NewReader(librawData))
	for scanner.Scan() {
		matchMaker := false
		matchModel := false
		matchAlias := false

		line := scanner.Text()

		if strings.Contains(line, "const model_map_t modelMap[] = {") {
			inStruct = true
			continue
		} else if inStruct == false {
			continue
		} else if strings.Contains(line, "};") && inStruct == true {
			break
		}

		matchMaker = strings.Contains(line, ".clean_make =")
		matchModel = strings.Contains(line, ".clean_model =")
		matchAlias = strings.Contains(line, ".clean_alias =")

		re := regexp.MustCompile(`".+"`)
		foundStr := strings.Trim(re.FindString(line), "\"")
		if matchMaker == true {
			maker = foundStr
		} else if matchModel == true {
			model = foundStr
		} else if matchAlias == true {
			alias = foundStr
		}

		if strings.Contains(line, "},") {
			key := strings.ToLower(maker + " " + model)
			camera := cameras[key]

			if model != alias {
				// Ensure no duplicate aliases
				aliasesCurrent := make(map[string]struct{})
				for _, a := range camera.Aliases {
					aliasesCurrent[strings.ToLower(a)] = struct{}{}
				}
				_, ok := aliasesCurrent[strings.ToLower(alias)]
				if ok == false {
					camera.Aliases = append(camera.Aliases, alias)
				}
			}

			camera.Decoder = "libraw"
			cameras[key] = camera
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal("Error occurred: ", err)
	}

	if inStruct == false {
		log.Fatal("No LibRaw cameras found in ", path)
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
				camera.Decoder = "unknown"
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
				camera.Decoder = "unknown"
			}
			camera.Maker = v.Maker
			camera.Model = m.Model
			camera.NoiseProfiles = true
			cameras[key] = camera
		}
	}
}

func generateStats(cameras map[string]camera, unsupported bool) stats {

	s := stats{}

	for _, c := range cameras {
		if c.Decoder == "" && unsupported == false {
			continue
		} else if c.Decoder == "" && unsupported == true {
			s.cameras += 1
			s.unsupported += 1
		} else if c.Decoder == "rawspeed" {
			s.cameras += 1
			s.rawspeed += 1
		} else if c.Decoder == "libraw" {
			s.cameras += 1
			s.libraw += 1
		} else if c.Decoder == "unknown" {
			s.cameras += 1
			s.unknown += 1
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

func generateMD(cameras map[string]camera, camerasOrder []string, unsupported bool) string {
	_ = cameras
	_ = camerasOrder
	_ = unsupported

	fmt.Println("Generate MD")
	return ""
}

func generateHTML(cameras map[string]camera, camerasOrder []string, unsupported bool) string {
	_ = cameras
	_ = camerasOrder
	_ = unsupported

	fmt.Println("Generate HTML")
	return ""
}

func generateTSV(cameras map[string]camera, camerasOrder []string, unsupported bool) string {
	_ = cameras
	_ = camerasOrder
	_ = unsupported

	fmt.Println("Generate TSV")
	return ""
}
