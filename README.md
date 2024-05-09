## Usage

`camera-support [-libraw <path>] [-rawspeed <path>] [-wbpresets <path>] [-noiseprofiles <path>] [-stats [stdout|table|all]] [-format <tsv|md|html>] [-headers <h1|h2|h3|h4|h5|h6>] [-fields <...>] [-out <path>]`

* **-libraw**
  
  `imageio_libraw.c` location. If omitted, LibRaw cameras are not added.
  URL or relative local path (default "data/imageio_libraw.c")
  
* **-rawspeed**

  rawspeed `cameras.xml` location.
  URL or relative local path (default "data/cameras.xml")

* **-wbpresets**

  `wb_presets.json` location.
  URL or relative local path (default "data/wb_presets.json")

* **-noiseprofiles**

  `noiseprofiles.json` location.
  URL or relative local path (default "data/noiseprofiles.json")
  
* **-stats**

  Print statistics. Default is `stdout` which prints at the end of normal output to the terminal.
  `table` adds stats to table headers.
  `all` does both.

* **-format**

  Output format. (default "md")
  
* **-headers**
  
  Segments tables by maker, adding a header using the specified level (h1-h6).
  
* **-fields**

  Comma delimited list of fields to print. Defaults to all.
  See the `camera` struct in `camera-support.go` for valid fields.

* **-out**

  Output file. Default is stdout.
