/*
   This file is part of darktable,
   Copyright (C) 2009-2020 darktable developers.

   darktable is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   darktable is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with darktable.  If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"bufio"
	"encoding/csv"
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
	// Having a map inside a struct is obnoxious, so this is not part of the options struct
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
		rawspeedDNGPath   string
		librawPath        string
		wbpresetsPath     string
		noiseprofilesPath string
		stats             string
		format            string
		thFormatStr       []string
		segments          int
		fields            []string
		bools             []string
		escape            bool
		unsupported       bool
		output            string
	}

	flag.StringVar(&options.rawspeedPath, "rawspeed", "https://raw.githubusercontent.com/darktable-org/rawspeed/develop/data/cameras.xml", "'cameras.xml' location.")
	flag.StringVar(&options.rawspeedDNGPath, "rawspeeddng", "./rawspeed-dng.csv", "'rawspeed-dng.csv' location.")
	flag.StringVar(&options.librawPath, "libraw", "https://raw.githubusercontent.com/darktable-org/darktable/master/src/imageio/imageio_libraw.c", "'imageio_libraw.c' location. If empty, LibRaw cameras will not be included.")
	flag.StringVar(&options.wbpresetsPath, "wbpresets", "https://raw.githubusercontent.com/darktable-org/darktable/master/data/wb_presets.json", "'wb_presets.json' location.")
	flag.StringVar(&options.noiseprofilesPath, "noiseprofiles", "https://raw.githubusercontent.com/darktable-org/darktable/master/data/noiseprofiles.json", "'noiseprofiles.json' location.")
	flag.StringVar(&options.stats, "stats", "stdout", "Print statistics. <stdout|table|all|none>")
	flag.StringVar(&options.format, "format", "md", "Output format. <md|tsv|none>")

	flag.Func("thformatstr", "Format string to use for header fields with statistics. Format is \"no-percent;with-percent\" with a semicolon delimiter. See Go's fmt docs for details.", func(s string) error {
		// Default is defined under unset flag handling
		if strings.Count(s, ";") != 1 {
			return errors.New("Must contain one semicolon")
		}
		options.thFormatStr = strings.Split(s, ";")
		return nil
	})

	flag.Func("segments", "Segments tables by maker, adding a header using the specified level. <1-6>", func(s string) error {
		i, err := strconv.Atoi(s)
		if err != nil || i > 6 {
			return errors.New("Must be an integer 0-6")
		}
		options.segments = i
		return nil
	})

	flag.Func("fields", "Semicolon delimited list of fields to print. See the 'camera' struct in 'camera-support.go' for valid fields. <...|no-maker|all|all-debug>", func(s string) error {
		// Default is defined under unset flag handling
		if s == "all" {
			s = "Maker;Model;Aliases;WBPresets;NoiseProfiles;Decoder;RSSupported;Formats"
		} else if s == "all-debug" {
			s = "Maker;Model;Aliases;WBPresets;NoiseProfiles;Decoder;RSSupported;Formats;Debug"
		} else if s == "no-maker" {
			s = "Model;Aliases;WBPresets;NoiseProfiles;Decoder"
		}
		s = strings.ToLower(s)
		options.fields = strings.Split(s, ";")
		return nil
	})

	flag.Func("bools", "Text to use for boolean fields. Format is \"true;false\" with a semicolon delimiter.", func(s string) error {
		// Default is defined under unset flag handling
		if strings.Count(s, ";") != 1 {
			return errors.New("Must contain one semicolon")
		}
		options.bools = strings.Split(s, ";")
		return nil
	})

	flag.BoolVar(&options.escape, "escape", false, "Escape Markdown characters in Model and Aliases fields.")
	flag.BoolVar(&options.unsupported, "unsupported", false, "Include unsupported cameras. Also affects statistics.")
	flag.Parse()

	// Handle unset flags
	if options.thFormatStr == nil {
		options.thFormatStr = append(options.thFormatStr, "%v (%v)", "%v (%v / %v%%)")
	}
	if options.fields == nil {
		options.fields = append(options.fields, "maker", "model", "aliases", "wbpresets", "noiseprofiles", "decoder")
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

	loadRawSpeedDNG(cameras, options.rawspeedDNGPath)

	stats := generateStats(cameras, options.unsupported)

	////  Output  ////

	if options.format != "none" {
		data := prepareOutputData(cameras, options.fields, options.bools, options.escape, options.unsupported)

		outputString := ""
		if options.format == "md" {
			outputString = generateMD(data, options.fields, columnHeaders, options.segments, options.stats, options.bools, options.thFormatStr)
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
			fmt.Println("")
		}
		fmt.Printf("Cameras:\t %4v\n", stats.cameras)
		fmt.Printf("  RawSpeed:\t %4v  %3v%%\n", stats.rawspeed, stats.rawspeedPercent)
		fmt.Printf("  LibRaw:\t %4v  %3v%%\n", stats.libraw, stats.librawPercent)
		fmt.Printf("  Unknown:\t %4v  %3v%%\n", stats.unknown, stats.unknownPercent)
		if options.unsupported == true {
			fmt.Printf("  Unsupported:\t %4v  %3v%%\n", stats.unsupported, stats.unsupportedPercent)
		}
		fmt.Printf("Aliases:\t %4v\n", stats.aliases)
		fmt.Printf("WB Presets:\t %4v  %3v%%\n", stats.wbPresets, stats.wbPresetsPercent)
		fmt.Printf("Noise Profiles:\t %4v  %3v%%\n", stats.noiseProfiles, stats.noiseProfilePercent)
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
		} else { // No <ID> element so get from <Camera>
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
			if camera.Maker == "" { // Camera isn't present in cameras.xml or imageio_libraw.c
				camera.Decoder = "Unknown"
				camera.Debug = append(camera.Debug, "Source: wb_presets.json")
			} else if camera.Decoder == "" {
				camera.Debug = append(camera.Debug, "wb_presets.json: No decoder")
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
			if camera.Maker == "" { // Camera isn't present in cameras.xml or imageio_libraw.c
				camera.Decoder = "Unknown"
				camera.Debug = append(camera.Debug, "Source: noiseprofiles.json")
			} else if camera.Decoder == "" {
				camera.Debug = append(camera.Debug, "noiseprofiles.json: No decoder")
			}
			camera.Maker = v.Maker
			camera.Model = m.Model
			camera.NoiseProfiles = true
			cameras[key] = camera
		}
	}
}

func loadRawSpeedDNG(cameras map[string]camera, rsDNGPath string) {
	rsDNG, err := os.Open(rsDNGPath)
	if err != nil {
		log.Fatal("Cannot open rawspeed-dng.csv: ", err)
	}
	defer rsDNG.Close()

	reader := csv.NewReader(rsDNG)
	// reader.Comma = ';'
	rows, err := reader.ReadAll()
	if err != nil {
		log.Fatal("Cannot read rawspeed-dng.csv: ", err)
	}

	for _, c := range rows {

		maker := c[0]
		model := c[1]
		key := strings.ToLower(maker + " " + model)

		if key == "maker model" { // Skip header line
			continue
		}

		camera, ok := cameras[key]
		if ok {
			camera.Decoder = "RawSpeed"
			camera.Debug = append(camera.Debug, "rawspeed-dng: Decoder set")
			cameras[key] = camera
		} else {
			log.Fatalln("rawspeed-dng.csv:", maker, model, "not found in cameras")
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
			switch f {
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

func generateMD(data [][]string, fields []string, colHeaders map[string]string, segments int, showStats string, bools []string, thFormatStr []string) string {

	headerFields := map[string][]string{}
	if showStats == "table" || showStats == "all" {
		sumModels := 0
		sumWB := 0
		sumNP := 0
		makerNext := ""
		rowsTotal := len(data)

		for i, r := range data {
			maker := r[1]
			if i != rowsTotal-1 {
				makerNext = data[i+1][1]
			}

			for j, f := range r {
				if j == 0 || j == 1 {
					continue
				}
				switch fields[j-2] {
				case "model":
					sumModels += 1
				case "wbpresets":
					if f == bools[0] {
						sumWB += 1
					}
				case "noiseprofiles":
					if f == bools[0] {
						sumNP += 1
					}
				}
			}

			if i == rowsTotal-1 || (segments != 0 && maker != makerNext) {
				percentWB := int(math.Round(float64(sumWB) / float64(sumModels) * 100))
				percentNP := int(math.Round(float64(sumNP) / float64(sumModels) * 100))

				hf := make([]string, 0, len(fields))
				for _, f := range fields {
					switch f {
					case "model":
						hf = append(hf, fmt.Sprintf(thFormatStr[0], colHeaders[f], sumModels))
					case "wbpresets":
						hf = append(hf, fmt.Sprintf(thFormatStr[1], colHeaders[f], sumWB, percentWB))
					case "noiseprofiles":
						hf = append(hf, fmt.Sprintf(thFormatStr[1], colHeaders[f], sumNP, percentNP))
					default:
						hf = append(hf, colHeaders[f])
					}
					if segments == 0 {
						headerFields["fulltable"] = hf
					} else {
						headerFields[maker] = hf
					}
				}

				sumModels = 0
				sumWB = 0
				sumNP = 0
			}
		}
	} else { // No stats
		hf := make([]string, 0, len(fields))
		for _, f := range fields {
			hf = append(hf, colHeaders[f])
		}
		headerFields["nostats"] = hf
	}

	// Calculate the widest field in each column, so table cells line up nicely
	colWidths := make([]int, len(fields), len(fields))
	for _, h := range headerFields {
		for i, f := range h {
			w := len(f)
			if w > colWidths[i] {
				colWidths[i] = w
			}
		}
	}
	for _, r := range data {
		for i, f := range r[2:] { // We skip the first two fields, since they are not in the output
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

	hLevel := strings.Repeat("#", segments)

	makerPrev := ""
	for i, r := range data {
		maker := r[1]

		if i == 0 && segments == 0 { // Table header
			if showStats == "table" || showStats == "all" {
				mdTable.WriteString(contructTableRow(headerFields["fulltable"], colWidths))
			} else {
				mdTable.WriteString(contructTableRow(headerFields["nostats"], colWidths))
			}
			mdTable.WriteString(tRowSep)
		}

		if segments != 0 && maker != makerPrev { // Segment header
			mdTable.WriteString(fmt.Sprintf("\n%s %s\n\n", hLevel, maker))
			if showStats == "table" || showStats == "all" {
				mdTable.WriteString(contructTableRow(headerFields[maker], colWidths))
			} else {
				mdTable.WriteString(contructTableRow(headerFields["nostats"], colWidths))
			}
			mdTable.WriteString(tRowSep)
		}

		mdTable.WriteString(contructTableRow(r[2:], colWidths))

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
		headers = append(headers, colHeaders[f])
	}

	tsvData := strings.Builder{}
	tsvData.WriteString(fmt.Sprintf("%v\n", strings.Join(headers, "\t")))
	for _, r := range data {
		tsvData.WriteString(fmt.Sprintf("%v\n", strings.Join(r[2:], "\t")))
	}

	return tsvData.String()
}
