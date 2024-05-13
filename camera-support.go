package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/beevik/etree"
)

type camera struct {
	Maker         string
	Model         string
	Aliases       []string
	Formats       []string // RawSpeed modes
	WBPresets     bool
	NoiseProfiles bool
	RSSupported   string // RawSpeed support
	Decoder       string // RawSpeed | LibRaw | Unknown
	Debug         []string
}

type stats struct {
	cameras             int
	aliases             int
	rawspeed            int
	rawspeedPercent     int
	libraw              int
	librawPercent       int
	unknown             int
	unknownPercent      int
	unsupported         int
	unsupportedPercent  int
	wbPresets           int
	wbPresetsPercent    int
	noiseProfiles       int
	noiseProfilePercent int
}

func main() {
	// Having a map inside a struct is obnoxious, so this is not part of options
	columnHeaders := map[string]string{
		"maker":         "Maker",
		"model":         "Model",
		"aliases":       "Aliases",
		"formats":       "Formats",
		"wbpresets":     "WB Presets",
		"noiseprofiles": "Noise Profile",
		"rssupported":   "RawSpeed Support",
		"decoder":       "Decoder",
		"debug":         "Debug",
	}

	var options struct {
		rawspeedPath      string
		librawPath        string
		wbpresetsPath     string
		noiseprofilesPath string
		stats             string
		format            string
		segments          int
		fields            []string
		bools             []string
		escape            bool
		unsupported       bool
		output            string
	}

	flag.StringVar(&options.rawspeedPath, "rawspeed", "https://raw.githubusercontent.com/darktable-org/rawspeed/develop/data/cameras.xml", "'cameras.xml' location.")
	flag.StringVar(&options.librawPath, "libraw", "https://raw.githubusercontent.com/darktable-org/darktable/master/src/imageio/imageio_libraw.c", "'imageio_libraw.c' location. If empty, LibRaw cameras will not be included.")
	flag.StringVar(&options.wbpresetsPath, "wbpresets", "https://raw.githubusercontent.com/darktable-org/darktable/master/data/wb_presets.json", "'wb_presets.json' location.")
	flag.StringVar(&options.noiseprofilesPath, "noiseprofiles", "https://raw.githubusercontent.com/darktable-org/darktable/master/data/noiseprofiles.json", "'noiseprofiles.json' location.")
	flag.StringVar(&options.stats, "stats", "stdout", "Print statistics. <stdout|table|all|none>")
	flag.StringVar(&options.format, "format", "md", "Output format. <md|tsv|none>")

	flag.Func("segments", "Segments tables by maker, adding a header using the specified level. <1-6>", func(s string) error {
		i, err := strconv.Atoi(s)
		if err != nil || i > 6 {
			return errors.New("Must be integer 0-6")
		}
		options.segments = i
		return nil
	})

	flag.Func("fields", "Comma delimited list of fields to print. See the 'camera' struct in 'camera-support.go' for valid fields. <...|no-maker|all|all-debug>", func(s string) error {
		// Default is defined under unset flag handling
		if s == "all" {
			s = "Maker,Model,Aliases,WBPresets,NoiseProfiles,Decoder,RSSupported,Formats"
		} else if s == "all-debug" {
			s = "Maker,Model,Aliases,WBPresets,NoiseProfiles,Decoder,RSSupported,Formats,Debug"
		} else if s == "no-maker" {
			s = "Model,Aliases,WBPresets,NoiseProfiles,Decoder"
		}
		options.fields = strings.Split(s, ",")
		return nil
	})

	flag.Func("bools", "Text to use for boolean fields. Format is \"true,false\" with a comma delimiter.", func(s string) error {
		// Default is defined under unset flag handling
		if strings.Count(s, ",") != 1 {
			return errors.New("Must contain one comma")
		}
		options.bools = strings.Split(s, ",")
		return nil
	})

	flag.BoolVar(&options.escape, "escape", false, "Escape Markdown characters in Model and Aliases fields.")
	flag.BoolVar(&options.unsupported, "unsupported", false, "Include unsupported cameras. Also affects statistics.")
	flag.Parse()

	// Handle unset flags
	if options.fields == nil {
		options.fields = append(options.fields, "Maker", "Model", "Aliases", "WBPresets", "NoiseProfiles", "Decoder")
	}
	if options.bools == nil {
		options.bools = append(options.bools, "Yes", "No")
	}

	// Non-flag options
	if flag.Arg(0) != "" {
		options.output = flag.Arg(0)
	} else {
		options.output = "stdout"
	}

	//// Logic ////

	cameras := map[string]camera{}

	loadRawSpeed(cameras, options.rawspeedPath, options.unsupported)

	if options.librawPath != "" {
		loadLibRaw(cameras, options.librawPath)
	}

	loadWBPresets(cameras, options.wbpresetsPath)
	loadNoiseProfiles(cameras, options.noiseprofilesPath)

	stats := generateStats(cameras, options.unsupported)

	////  Output  ////

	if options.format != "none" {
		data := prepareOutputData(cameras, options.fields, options.bools, options.escape, options.unsupported)

		outputString := ""
		if options.format == "md" {
			outputString = generateMD(data, options.fields, columnHeaders, options.segments, options.stats, stats)
		} else if options.format == "tsv" {
			outputString = generateTSV(data, options.fields, columnHeaders)
		} else {
			log.Fatalf("Invalid format string: %v\n", options.format)
		}

		if options.output != "stdout" {
			if err := os.WriteFile(options.output, []byte(outputString), 0666); err != nil {
				log.Fatal(err)
			}
		} else {
			fmt.Print(outputString)
		}
	}

	if options.stats == "stdout" || options.stats == "all" {
		if options.output == "stdout" && options.format != "none" {
			fmt.Println("\r")
		}
		fmt.Printf("Cameras:\t %v\n", stats.cameras)
		fmt.Printf("  RawSpeed:\t %v (%v%%)\n", stats.rawspeed, stats.rawspeedPercent)
		fmt.Printf("  LibRaw:\t %v (%v%%)\n", stats.libraw, stats.librawPercent)
		fmt.Printf("  Unknown:\t %v (%v%%)\n", stats.unknown, stats.unknownPercent)
		fmt.Printf("  Unsupported:\t %v (%v%%)\n", stats.unsupported, stats.unsupportedPercent)
		fmt.Printf("Aliases:\t %v\n", stats.aliases)
		fmt.Printf("WB Presets:\t %v (%v%%)\n", stats.wbPresets, stats.wbPresetsPercent)
		fmt.Printf("Noise Profiles:\t %v (%v%%)\n", stats.noiseProfiles, stats.noiseProfilePercent)
	}
}

func getData(path string) []byte {
	if strings.HasPrefix(path, "https://") {
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
		data, err := os.ReadFile(path)
		if err != nil {
			log.Fatal(err)
		}
		return data
	}
}

func loadRawSpeed(cameras map[string]camera, path string, unsupported bool) {
	camerasXML := etree.NewDocument()
	if err := camerasXML.ReadFromBytes(getData(path)); err != nil {
		log.Fatal(err)
	}

	root := camerasXML.SelectElement("Cameras")
	for _, c := range root.SelectElements("Camera") {
		maker := ""
		model := ""
		debug := make([]string, 0, 3)
		key := ""

		if id := c.SelectElement("ID"); id != nil {
			maker = id.SelectAttrValue("make", "")
			model = id.SelectAttrValue("model", "")
			key = strings.ToLower(maker + " " + model)
		} else { // No ID element so get from Camera
			maker = c.SelectAttrValue("make", "")
			model = c.SelectAttrValue("model", "")
			key = strings.ToLower(maker + " " + model)

			if model == "" {
				debug = append(debug, "cameras.xml: No Model in Camera element")
			}
		}

		camera := cameras[key]
		camera.Maker = maker
		camera.Model = model

		if aliases := c.SelectElement("Aliases"); aliases != nil {
			for _, a := range aliases.SelectElements("Alias") {
				alias := ""
				id := a.SelectAttrValue("id", "")
				val := a.Text()
				if id == "" {
					// Sometimes <Alias> doesn't have an id attribute, so use the text instead
					// Not ideal, but probably the best that can be done for now
					// Would be better if cameras.xml was consistent
					alias, _ = strings.CutPrefix(val, maker+" ")
					debug = append(debug, "cameras.xml: No id in Alias")
				} else {
					alias = id
				}
				// fmt.Println("  id:\t" + id)
				// fmt.Println("  val:\t" + val)
				// fmt.Println("  alias:\t" + alias)
				camera.Aliases = append(camera.Aliases, alias)
				slices.Sort(camera.Aliases)
				camera.Aliases = slices.Compact(camera.Aliases)
			}
		}

		if format := c.SelectAttrValue("mode", ""); format != "" {
			camera.Formats = append(camera.Formats, format)
		} //  else {
		// 	camera.Formats = append(camera.Formats, "default")
		// }

		camera.RSSupported = c.SelectAttrValue("supported", "")
		if camera.RSSupported != "" && unsupported == false {
			continue
		}
		if camera.RSSupported == "" {
			camera.Decoder = "RawSpeed"
		}

		camera.Debug = append(camera.Debug, debug...)
		slices.Sort(camera.Debug)
		camera.Debug = slices.Compact(camera.Debug)

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

			camera.Maker = maker
			camera.Model = model
			camera.Decoder = "LibRaw"
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
				camera.Decoder = "Unknown"
				camera.Debug = append(camera.Debug, "Source: wb_presets.json")
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
				camera.Decoder = "Unknown"
				camera.Debug = append(camera.Debug, "Source: noiseprofiles.json")
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

	// Totals
	for _, c := range cameras {
		if c.Decoder == "" && unsupported == false {
			continue
		} else if c.Decoder == "" && unsupported == true {
			s.unsupported += 1
		} else if c.Decoder == "RawSpeed" {
			s.rawspeed += 1
		} else if c.Decoder == "LibRaw" {
			s.libraw += 1
		} else if c.Decoder == "Unknown" {
			s.unknown += 1
		}

		s.aliases += len(c.Aliases)

		if c.NoiseProfiles == true {
			s.noiseProfiles += 1
		}

		if c.WBPresets == true {
			s.wbPresets += 1
		}

		s.cameras += 1
	}

	// Percentages
	s.rawspeedPercent = int(math.Round(float64(s.rawspeed) / float64(s.cameras) * 100))
	s.librawPercent = int(math.Round(float64(s.libraw) / float64(s.cameras) * 100))
	s.unknownPercent = int(math.Round(float64(s.unknown) / float64(s.cameras) * 100))
	s.unsupportedPercent = int(math.Round(float64(s.unsupported) / float64(s.cameras) * 100))
	s.wbPresetsPercent = int(math.Round(float64(s.wbPresets) / float64(s.cameras) * 100))
	s.noiseProfilePercent = int(math.Round(float64(s.noiseProfiles) / float64(s.cameras) * 100))

	return s
}

func prepareOutputData(cameras map[string]camera, fields []string, bools []string, escape bool, unsupported bool) [][]string {
	data := make([][]string, 0, len(cameras))

	mdEscapes := strings.NewReplacer(
		"\\", "\\\\",
		"*", "\\*",
		"_", "\\_",
		"{", "\\{",
		"}", "\\}",
		"[", "\\[",
		"]", "\\]",
		"<", "\\<",
		">", "\\>",
		"(", "\\(",
		")", "\\)",
		"#", "\\#",
	)

	// Maps can't be sorted, so use a separate sorted slice for the output order
	camerasOrder := make([]string, 0, len(cameras))
	for k := range cameras {
		camerasOrder = append(camerasOrder, k)
	}
	sort.Strings(camerasOrder)

	for _, k := range camerasOrder {
		c := cameras[k]

		if unsupported == false && c.Decoder == "" {
			continue
		}

		// First two fields in row are always cameras key and Maker, even if not requested
		// They may be needed when generating the output
		row := make([]string, 0, len(fields)+2)
		row = append(row, k)
		row = append(row, c.Maker)

		for _, f := range fields {
			switch strings.ToLower(f) {
			case "maker":
				row = append(row, c.Maker)
			case "model":
				if escape == true {
					row = append(row, mdEscapes.Replace(c.Model))
				} else {
					row = append(row, c.Model)
				}
			case "aliases":
				if escape == true {
					row = append(row, mdEscapes.Replace(strings.Join(c.Aliases, ", ")))
				} else {
					row = append(row, strings.Join(c.Aliases, ", "))
				}
			case "formats":
				row = append(row, strings.Join(c.Formats, ", "))
			case "wbpresets":
				if c.WBPresets == true {
					row = append(row, bools[0])
				} else {
					row = append(row, bools[1])
				}
			case "noiseprofiles":
				if c.NoiseProfiles == true {
					row = append(row, bools[0])
				} else {
					row = append(row, bools[1])
				}
			case "rssupported":
				row = append(row, c.RSSupported)
			case "decoder":
				row = append(row, c.Decoder)
			case "debug":
				row = append(row, strings.Join(c.Debug, ", "))
			}
		}

		data = append(data, row)
	}

	return data
}

func generateMD(data [][]string, fields []string, colHeaders map[string]string, segments int, showStats string, stats stats) string {
	_ = stats
	_ = showStats

	// Calculate the widest field in each column, so table cells line up nicely
	colWidths := make([]int, 0, len(fields))
	for _, f := range fields {
		w := len(colHeaders[strings.ToLower(f)])
		colWidths = append(colWidths, w)
	}
	for _, r := range data {
		for i, f := range r[2:] {
			w := len(f)
			if w > colWidths[i] {
				colWidths[i] = w
			}
		}
	}

	// Table row separator
	sep := make([]string, 0, len(colWidths))
	for _, c := range colWidths {
		sep = append(sep, strings.Repeat("-", c))
	}
	tRowSep := contructTableRow(sep, colWidths)

	// Build the table
	mdTable := strings.Builder{}

	tHeaders := make([]string, 0, len(fields))
	for _, f := range fields {
		tHeaders = append(tHeaders, colHeaders[strings.ToLower(f)])
	}
	tHeaderRow := contructTableRow(tHeaders, colWidths)

	hLevel := strings.Repeat("#", segments)

	makerPrev := ""
	for i, r := range data {
		maker := r[1]

		if i == 0 && segments == 0 { // Table header
			mdTable.WriteString(tHeaderRow)
			mdTable.WriteString(tRowSep)
		}

		if segments != 0 && maker != makerPrev { // Segment header
			mdTable.WriteString(fmt.Sprintf("\n%s %s\n\n", hLevel, maker))
			mdTable.WriteString(tHeaderRow)
			mdTable.WriteString(tRowSep)
		}

		tRow := contructTableRow(r[2:], colWidths)
		mdTable.WriteString(tRow)

		makerPrev = maker
	}

	return mdTable.String()
}

func contructTableRow(fields []string, colWidths []int) string {
	tableRow := strings.Builder{}

	for i, f := range fields {
		tableRow.WriteString(fmt.Sprintf("| %-*s ", colWidths[i], f))
		if i == len(fields)-1 {
			tableRow.WriteString(fmt.Sprintf("|\n"))
		}
	}
	return tableRow.String()
}

func generateTSV(data [][]string, fields []string, colHeaders map[string]string) string {
	headers := make([]string, 0, len(fields))
	for _, f := range fields {
		headers = append(headers, colHeaders[strings.ToLower(f)])
	}

	tsvData := strings.Builder{}
	tsvData.WriteString(fmt.Sprintf("%v\n", strings.Join(headers, "\t")))
	for _, r := range data {
		tsvData.WriteString(fmt.Sprintf("%v\n", strings.Join(r[2:], "\t")))
	}

	return tsvData.String()
}
