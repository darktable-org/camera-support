# Darktable Camera Support

Generate list of cameras supported by darktable.

By default the list will be pulled from the current development version, so may not reflect what the stable version supports.

## Usage

`camera-support [-libraw <path>] [-rawspeed <path>] [-wbpresets <path>] [-noiseprofiles <path>] [-stats <stdout|table|all|none>] [-format <md|tsv|none>] [-thformatstr <...;...>] [-segments <1-6>] [-fields <...|no-maker|all|all-debug>] [-bools <...,...>] [-escape] [-unsupported] [<output path>]`

All options that take a file path accept either a URL (starting with `https://`) or a relative local path.

### -libraw

`imageio_libraw.c` location. If empty, LibRaw cameras will not be included.
Default: `https://raw.githubusercontent.com/darktable-org/darktable/master/src/imageio/imageio_libraw.c`

### -rawspeed

`cameras.xml` location.
Default: `https://raw.githubusercontent.com/darktable-org/rawspeed/develop/data/cameras.xml`

### -wbpresets

`wb_presets.json` location.
Default: `https://raw.githubusercontent.com/darktable-org/darktable/master/data/wb_presets.json`

### -noiseprofiles

`noiseprofiles.json` location.
Default: `https://raw.githubusercontent.com/darktable-org/darktable/master/data/noiseprofiles.json`

### -stats

Print statistics.
The default is `stdout` which prints at the end of normal output to the terminal.
`table` adds stats to table headers.
`all` does both.
`none` prints nothing.

### -format

Output format.
`md`, the default, is Markdown table.
`tsv` is tab separated values.
`none` creates no output. Useful if only interested in statistics.

### -thformatstr

Format string to use for table headers with statistics. Format is `no-percent;with-percent` with a semicolon delimiter. Default is `%v (%v);%v (%v / %v%%)`.
See Go's fmt docs for details: https://pkg.go.dev/fmt  
Also accepts Markdown formatting allowed in tables.

### -segments

Segments tables by maker, adding a header using the specified level (1-6).

### -fields

Comma delimited list of fields to print. Default is `Maker,Model,Aliases,WBPresets,NoiseProfiles,Decoder`.
See the `camera` struct in `camera-support.go` for valid fields. Not case-sensitive.
Presets: `no-maker|all|all-debug`

### -bools

Text to use for boolean fields. Format is `true,false` with a comma delimiter. Accepts Markdown formatting allowed in tables.

### -escape

Escape Markdown characters in Model and Aliases fields.

### -unsupported

Include unsupported cameras. Also affects statistics.

### \<output path\>

Output file. Defaults to stdout.

### -h / -help

Prints a short version of this help.
