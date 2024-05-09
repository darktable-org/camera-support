## Usage

`camera-support [-libraw <path>] [-rawspeed <path>] [-wbpresets <path>] [-noiseprofiles <path>] [-stats [stdout|table|all]] [-format <tsv|md|html>] [-headers <h1|h2|h3|h4|h5|h6>] [-fields <...>] [-unsupported] [-out <path>]`

All options that take a file path accept either a URL (starting with `https://`) or a relative local path.

### -libraw
  
`imageio_libraw.c` location. If omitted, LibRaw cameras are not added.
Default: `https://github.com/darktable-org/darktable/blob/master/src/imageio/imageio_libraw.c`
  
### -rawspeed

rawspeed `cameras.xml` location.
Default: `https://github.com/darktable-org/rawspeed/blob/develop/data/cameras.xml`

### -wbpresets

`wb_presets.json` location.
Default: `https://github.com/darktable-org/darktable/blob/master/data/wb_presets.json`

### -noiseprofiles

`noiseprofiles.json` location.
Default: `https://github.com/darktable-org/darktable/blob/master/data/noiseprofiles.json`
  
### -stats

Print statistics. Default is `stdout` which prints at the end of normal output to the terminal.
`table` adds stats to table headers.
`all` does both.

### -format

Output format.
`md`, the default, is Markdown table.
`html` is HTML table.
`tsv` is tab separated values.
  
### -headers
  
Segments tables by maker, adding a header using the specified level (h1-h6).
  
### -fields

Comma delimited list of fields to print. Default is all.
See the `camera` struct in `camera-support.go` for valid fields.

### -unsupported

Include unsupported cameras. Also affects statistics.

### -out

Output file. Default is stdout.

### -h / -help

Prints a short version of this help.
