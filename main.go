package main

// MODIFIED TO OUTPUT STATE CENTERS AS A CSV
import (
	"bufio"
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang/freetype"
	bmp "github.com/jsummers/gobmp"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"gopkg.in/yaml.v2"
)

var modPath = "e:/Mod Repository/Hearts of Iron IV/mod/oldworldblues"
var configPath = "config.yml"
var hoi4Path string

// var modPath = "d:/Games/SteamApps/common/Hearts of Iron IV"
var definitionsPath = modPath + "/map/definition.csv"
var adjacenciesPath = modPath + "/map/adjacencies.csv"
var provincesPath = modPath + "/map/provinces.bmp"
var terrainPath = modPath + "/map/terrain.bmp"
var heightmapPath = modPath + "/map/heightmap.bmp"
var statesPath = modPath + "/history/states"
var strategicRegionPath = modPath + "/map/strategicregions"
var localModFilesPath = "./mod_path/"
var gfxFilePath = localModFilesPath + "gfx/interface/gui/custom_map_mode"
var atlasPath = "atlas0.png"
var provincesIDMap = make(map[int]*Province)
var provincesRGBMap = make(map[color.Color]*Province)
var statesMap = make(map[int]*State)
var strategicRegionMap = make(map[int]*StrategicRegion)
var rComment = regexp.MustCompile(`(#.*)`)
var rStateID = regexp.MustCompile(`(?:id[ \n\t]*?=[ \n\t]*?(\d+))`)
var rStateName = regexp.MustCompile(`(?:name[ \n\t]*?=[ \n\t]*?\"(.+?)\")`)
var rStateManpower = regexp.MustCompile(`(?:manpower[ \n\t]*?=[ \n\t]*?(\d+))`)
var rStateProvinces = regexp.MustCompile(`(?s:provinces[ \n\t]*?=[ \n\t]*?{.*?([0-9 \s]+).*?})`)
var rStateInfrastructure = regexp.MustCompile(`(?:infrastructure[ \n\t]*?=[ \n\t]*?(\d+))`)
var rStateImpassable = regexp.MustCompile(`(?:impassable[ \n\t]*?=[ \n\t]*?yes)`)
var rSpace = regexp.MustCompile(`\s+`)
var mapScalePixelToKm = 7.114
var provincesImageSize image.Rectangle
var waterColor = color.RGBA{68, 107, 163, 255}
var atlasTileSize = image.Rectangle{image.Point{0, 0}, image.Point{512, 512}}

/// Offsets for each texture index
var atlasTextureMap = []image.Point{
	image.Pt(0, 0), image.Pt(512, 0), image.Pt(1024, 0), image.Pt(1536, 0),
	image.Pt(0, 512), image.Pt(512, 512), image.Pt(1024, 512), image.Pt(1536, 512),
	image.Pt(0, 1024), image.Pt(512, 1024), image.Pt(1024, 1024), image.Pt(1536, 1024),
	image.Pt(0, 1536), image.Pt(512, 1536), image.Pt(1024, 1536), image.Pt(1536, 1536),
}
var charWidth = 4
var charHeight = 5
var startTime time.Time
var utf8bom = []byte{0xEF, 0xBB, 0xBF}

//go:embed smallest_pixel-7.ttf
var fontBytes []byte

//go:embed alpha_mask.png
var alphaMaskImage []byte

//go:embed vanilla_terrain.png
var vanilla_terrain_file []byte

//var image_Size = 0.5
var image_repeats int = 5
var image_repeats_hash int = 3
var colors = []color.NRGBA{}
var hash_colors = []color.NRGBA{}
var hash_scale float64 = 1.0
var main_scale float64 = 1.0
var readVanillaStates bool = false

type Config struct {
	ModPath           string  `yaml:"modPath"`
	HoiPath           string  `yaml:"hoi4Path"`
	MainScale         float64 `yaml:"mainScale"`
	HashScale         float64 `yaml:"hashScale"`
	ReadVanillaStates bool    `yaml:"readVanillaStates"`
	Color             []struct {
		R uint8 `yaml:"r"`
		G uint8 `yaml:"g"`
		B uint8 `yaml:"b"`
		A uint8 `yaml:"a"`
	} `yaml:"colors"`
	HashColor []struct {
		R uint8 `yaml:"r"`
		G uint8 `yaml:"g"`
		B uint8 `yaml:"b"`
		A uint8 `yaml:"a"`
	} `yaml:"hashColors"`
}

var defaultConfig = `
modPath: "C:/Program Files (x86)/Steam/steamapps/common/Hearts of Iron IV"
hoi4Path: "C:/Program Files (x86)/Steam/steamapps/common/Hearts of Iron IV"
mainScale: 1.0
hashScale: 1.0
readVanillaStates: false
colors:
  - color:
    r: 24
    g: 255
    b: 214
    a: 55
  - color:
    r: 135
    g: 216
    b: 148
    a: 55
  - color:
    r: 226
    g: 156
    b: 22
    a: 55
  - color:
    r: 34
    g: 69
    b: 255
    a: 55
  - color:
    r: 255
    g: 34
    b: 238
    a: 55
hashColors:
  - color:
    r: 106
    g: 228
    b: 3
    a: 255
  - color:
    r: 255
    g: 245
    b: 34
    a: 255
  - color:
    r: 255
    g: 34
    b: 34
    a: 255
`

// Province represents an in-game province with all parsed data in it.
type Province struct {
	ID              int
	RGB             color.RGBA
	Type            string // "land", "sea" or "lake"
	IsCoastal       bool
	Terrain         string
	Continent       int
	State           *State
	StrategicRegion *StrategicRegion
	PixelCoords     []image.Point
	PixelCoordsMap  map[image.Point]bool
	CenterPoint     image.Point
	AdjacentTo      map[int]*Province
	ConnectedTo     map[int]*Province
	ImpassableTo    map[int]*Province
	RenderColor     color.RGBA
}

var StateSizeFactor float64 = 60

// State represents an in-game state with all parsed data in it.
type State struct {
	ID             int
	Name           string
	Manpower       int
	Infrastructure int
	IsCoastal      bool
	IsImpassable   bool
	Continent      int
	PixelCoords    []image.Point
	PixelCoordsMap map[image.Point]bool
	CenterPoint    image.Point
	CenterPointRec image.Point
	StateSize      float64
	Provinces      map[int]*Province
	DistanceTo     map[int]int // Distance to other states.
	AdjacentTo     map[int]*State
	ConnectedTo    map[int]*State
	ImpassableTo   map[int]*State
	RenderColor    color.RGBA
}

// StrategicRegion represents an in-game strategic_region with all parsed data in it.
type StrategicRegion struct {
	ID             int
	Name           string
	Provinces      map[int]*Province
	PixelCoords    []image.Point
	PixelCoordsMap map[image.Point]bool
	CenterPoint    image.Point
}

var TerrainAtlas map[int]*TerrainType

type TerrainType struct {
	color         int
	texture_index int
	texture_img   image.Image
}

func processError(err error) {
	fmt.Printf("Error: %v", err)
	fmt.Scanln()
	panic(err)
}
func (c *Config) getConf() *Config {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultFile, err := os.Create(configPath)
		if err != nil {
			processError(err)
		}
		defer defaultFile.Close()
		_, err = defaultFile.WriteString(defaultConfig)
		if err != nil {
			processError(err)
		}
		defaultFile.Sync()
		fmt.Printf("Creating Config File, please exit and edit to preference")
		fmt.Scanln()
		os.Exit(0)
	}
	yamlFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		processError(err)
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		processError(err)
	}

	return c
}

func main() {
	// Track start time for benchmarking.
	startTime = time.Now()
	// Parse Config
	var cfg Config
	cfg.getConf()
	for _, c := range cfg.Color {
		colorNRGBA := color.NRGBA{c.R, c.G, c.B, c.A}
		colors = append(colors, colorNRGBA)
	}
	for _, ch := range cfg.HashColor {
		colorNRGBA := color.NRGBA{ch.R, ch.G, ch.B, ch.A}
		hash_colors = append(hash_colors, colorNRGBA)
	}
	modPath = cfg.ModPath
	hoi4Path = cfg.HoiPath
	readVanillaStates = cfg.ReadVanillaStates
	if cfg.MainScale > 0.01 {
		main_scale = cfg.MainScale
	}
	if cfg.HashScale > 0.01 {
		hash_scale = cfg.HashScale
	}
	fmt.Printf("Path to game: %v\n", hoi4Path)
	fmt.Printf("Path to mod: %v\n", modPath)
	if readVanillaStates {
		fmt.Print("Utilizing vanilla history files when missing states\n")
	}
	fmt.Printf("Hash Colors: %v\n", hash_colors)
	fmt.Printf("Main Colors %v\n", colors)
	getModPaths(modPath, hoi4Path)
	image_repeats = len(colors)
	image_repeats_hash = len(hash_colors)

	// Parse  definition.csv for provinces.
	err := parseDefinitions()
	if err != nil {
		processError(err)
	}

	// Parse  adjacencies.csv for province connections and impassable borders.
	err = parseAdjacencies()
	if err != nil {
		processError(err)
	}

	// Parse provinces.bmp for province adjacency.
	err = parseProvinces()
	if err != nil {
		processError(err)
	}

	// Find the center points of each province.
	findProvincesCenterPoints()

	// Parse state files.
	err = parseStateFiles()
	if err != nil {
		processError(err)
	}

	// Parse states provinces.
	parseStatesProvinces()

	// Parse states distance to other states.
	parseStatesDistanceToOtherStates()

	// Parse state files.
	err = parseStrategicRegionFiles()
	if err != nil {
		processError(err)
	}

	// Parse strategic regions provinces.
	parseStrategicRegionsProvinces()
	text := ""
	if stringContains(os.Args, "debug") {
		fmt.Print("Debug Mode\n")
		text = "createCustomMapFiles"
	} else {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print(`Enter maps to generate:
		generateTerrainMaps
		createCustomMapFiles
		generateStateMaps
		generateProvinceMaps
		--------------------

		`)
		text, _ = reader.ReadString('\n')
	}
	err = saveGeoData()
	if err != nil {
		processError(err)
	}
	// // Generate state ID map.
	if strings.Contains(text, "generateStateMaps") {
		err = generateSateMap()
		if err != nil {
			processError(err)
		}
		err = generateColoredSateMap()
		if err != nil {
			processError(err)
		}
		err = generateSateIDMap()
		if err != nil {
			processError(err)
		}
		err = generateInfrastructureMap()
		if err != nil {
			processError(err)
		}
		// Generate manpower map.
		err = generateManpowerMap()
		if err != nil {
			processError(err)
		}
	}
	if strings.Contains(text, "generateProvinceMaps") {
		err = generateProvinceMap()
		if err != nil {
			processError(err)
		}
		err = generateProvinceIDMap()
		if err != nil {
			processError(err)
		}
		err = generateSeaProvinceMap()
		if err != nil {
			processError(err)
		}
		err = generateSmallProvincesMap(32)
		if err != nil {
			processError(err)
		}
	}
	if strings.Contains(text, "generateTerrainMaps") {
		err = generateProvinceBasedHeightmapThresholdMap()
		if err != nil {
			processError(err)
		}
		err = generateProvinceBasedTerrainMap()
		if err != nil {
			processError(err)
		}
		err = generateImpassableMap()
		if err != nil {
			panic(err)
		}
	}
	if strings.Contains(text, "createCustomMapFiles") {
		// Write the output file.
		err = createStatePngFiles()
		if err != nil {
			processError(err)
		}
		err = createStateCenterPointsCSV()
		if err != nil {
			processError(err)
		}

		// ####################################################
		// Disabled for this patch of the tool - Coming soon
		// ####################################################

		// createDebugTerrainType()
		// err = parseTerrainTypes()
		// if err != nil {
		// 	processError(err)
		// }
		// err = createStateBackdrop()
		// if err != nil {
		// 	processError(err)
		// }
	}

	// // Generate province-based terrain map.
	// err = generateProvinceBasedTerrainMap()
	// if err != nil {
	// 	panic(err)
	// }

	// // Generate province-based heightmap threshold map.
	// err = generateProvinceBasedHeightmapThresholdMap()
	// if err != nil {
	// 	panic(err)
	// }

	// // Generate color shuffled province map.
	// err = generateColorShuffledProvinceMap()
	// if err != nil {
	// 	panic(err)
	// }

	// // Generate state ID map.
	// err = generateProvinceContinentValues("continents.png")
	// if err != nil {
	// 	panic(err)
	// }

	// // Generate impassable terrain map.
	// err = generateImpassableMap()
	// if err != nil {
	// 	panic(err)
	// }

	// Print out elapsed time.
	elapsedTime := time.Since(startTime)
	fmt.Printf("Elapsed time: %s\n", elapsedTime)
	if stringContains(os.Args, "debug") {
		fmt.Print("Debug Complete \n")
	} else {
		fmt.Scanln()
	}
}
func getModPaths(pathToInstall string, pathToHoi4 string) {
	definitionsPath = checkPath("/map/definition.csv", pathToInstall, pathToHoi4)
	adjacenciesPath = checkPath("/map/adjacencies.csv", pathToInstall, pathToHoi4)
	provincesPath = checkPath("/map/provinces.bmp", pathToInstall, pathToHoi4)
	terrainPath = checkPath("/map/terrain.bmp", pathToInstall, pathToHoi4)
	heightmapPath = checkPath("/map/heightmap.bmp", pathToInstall, pathToHoi4)
	statesPath = checkPath("/history/states", pathToInstall, pathToHoi4)
	strategicRegionPath = checkPath("/map/strategicregions", pathToInstall, pathToHoi4)
}

func checkPath(path string, pathToInstall string, pathToHoi4 string) string {
	fullPath := pathToInstall + path
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		fullPath = pathToHoi4 + path
		fmt.Printf("Mod version of %v not found, utilizing %v \n", path, fullPath)
	}
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		fmt.Printf("Could not find %v Exiting \n", fullPath)
		processError(err)
	}
	return fullPath
}

func stringContains(s []string, e string) bool {
	for _, a := range s {
		if strings.Contains(a, e) {
			return true
		}
	}
	return false
}

// ReadLines reads a whole file
// and returns a slice of its lines.
func readLines(path string) ([]string, error) {
	_, err := os.Stat(path)
	if err != nil {
		processError(err)
	}
	file, err := os.Open(path)
	if err != nil {
		processError(err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Remove utf-8 bom if found.
	if len(lines) > 0 {
		utf8bomString := string(utf8bom)
		if strings.HasPrefix(lines[0], utf8bomString) {
			lines[0] = strings.TrimPrefix(lines[0], utf8bomString)
		}
	}

	return lines, scanner.Err()
}

func parseDefinitions() error {
	fmt.Printf("%s: Parsing definition.csv...\n", time.Since(startTime))
	definitions, err := readLines(filepath.FromSlash(definitionsPath))
	if err != nil {
		return err
	}

	for _, s := range definitions {
		province, err := parseDefinitionsProvince(s)
		if err != nil {
			return err
		}
		provincesIDMap[province.ID] = &province
		provincesRGBMap[province.RGB] = &province
	}
	return nil
}

func parseDefinitionsProvince(s string) (p Province, err error) {
	pStrings := strings.Split(s, ";")
	if len(pStrings) != 8 {
		return p, errors.New("\"" + definitionsPath + "\": " + s + ": must contain 8 fields :: id:")
	}

	p.ID, err = strconv.Atoi(pStrings[0])
	if err != nil {
		return p, err
	}
	r, err := strconv.Atoi(pStrings[1])
	if err != nil {
		return p, err
	}
	g, err := strconv.Atoi(pStrings[2])
	if err != nil {
		return p, err
	}
	b, err := strconv.Atoi(pStrings[3])
	if err != nil {
		return p, err
	}
	p.RGB = color.RGBA{uint8(r), uint8(g), uint8(b), 255}
	p.Type = pStrings[4]
	p.IsCoastal, err = strconv.ParseBool(pStrings[5])
	if err != nil {
		return p, err
	}
	p.Terrain = pStrings[6]
	p.Continent, err = strconv.Atoi(pStrings[7])
	if err != nil {
		return p, err
	}
	p.PixelCoordsMap = make(map[image.Point]bool)
	p.AdjacentTo = make(map[int]*Province)
	p.ConnectedTo = make(map[int]*Province)
	p.ImpassableTo = make(map[int]*Province)

	return p, nil
}

func parseAdjacencies() error {
	fmt.Printf("%s: Parsing adjacencies.csv...\n", time.Since(startTime))
	adjacencies, err := readLines(filepath.FromSlash(adjacenciesPath))
	if err != nil {
		return err
	}
	// Skip first and last lines.
	for _, s := range adjacencies[1 : len(adjacencies)-1] {
		err := parseAdjacenciesState(s)
		if err != nil {
			return err
		}
	}
	return nil
}

func parseAdjacenciesState(s string) error {
	// Skip commented and empty lines.
	if strings.HasPrefix(s, "#") || len(s) == 0 {
		return nil
	}

	a := strings.Split(s, ";")
	if len(a) != 10 {
		return errors.New("\"" + adjacenciesPath + "\": " + s + ": must contain 10 fields")
	}

	id1, err := strconv.Atoi(a[0])
	if err != nil {
		return err
	}
	id2, err := strconv.Atoi(a[1])
	if err != nil {
		return err
	}

	if a[2] == "sea" || a[2] == "" {
		provincesIDMap[id1].ConnectedTo[id2] = provincesIDMap[id2]
		provincesIDMap[id2].ConnectedTo[id1] = provincesIDMap[id1]
	}

	if a[2] == "impassable" {
		provincesIDMap[id1].ImpassableTo[id2] = provincesIDMap[id2]
		provincesIDMap[id2].ImpassableTo[id1] = provincesIDMap[id1]
	}

	return nil
}

func parseProvinces() error {
	fmt.Printf("%s: Parsing provinces.bmp...\n", time.Since(startTime))
	provincesFile, err := os.Open(filepath.FromSlash(provincesPath))
	if err != nil {
		return err
	}
	defer provincesFile.Close()
	provincesImage, err := bmp.Decode(provincesFile)
	if err != nil {
		return err
	}

	provincesImageSize.Max = image.Point{provincesImage.Bounds().Max.X, provincesImage.Bounds().Max.Y}

	// Parse each pixel in scanline order.
	for y := 0; y < provincesImage.Bounds().Max.Y; y++ {
		for x := 0; x < provincesImage.Bounds().Max.X; x++ {
			var e, s color.Color

			// Get the color of the current pixel.
			c := provincesImage.At(x, y)
			r, g, b, a := c.RGBA()
			c = color.RGBA{uint8(r), uint8(g), uint8(b), uint8(a)}

			// Add pixel coordinates to the province that has this RGB value.
			provincesRGBMap[c].PixelCoordsMap[image.Point{x, y}] = true
			provincesRGBMap[c].PixelCoords = append(provincesRGBMap[c].PixelCoords, image.Point{x, y})

			// Find out the color of the adjacent right and bottom pixels.
			if x < provincesImage.Bounds().Max.X-1 {
				e = provincesImage.At(x+1, y)
				r, g, b, a := e.RGBA()
				e = color.RGBA{uint8(r), uint8(g), uint8(b), uint8(a)}
			}
			if y < provincesImage.Bounds().Max.Y-1 {
				s = provincesImage.At(x, y+1)
				r, g, b, a := s.RGBA()
				s = color.RGBA{uint8(r), uint8(g), uint8(b), uint8(a)}
			}

			// If color is different then this two provinces are adjacent.
			if (c != e) && (e != nil) {
				provincesRGBMap[c].AdjacentTo[provincesRGBMap[e].ID] = provincesRGBMap[e]
				provincesRGBMap[e].AdjacentTo[provincesRGBMap[c].ID] = provincesRGBMap[c]
			}
			if (c != s) && (s != nil) {
				provincesRGBMap[c].AdjacentTo[provincesRGBMap[s].ID] = provincesRGBMap[s]
				provincesRGBMap[s].AdjacentTo[provincesRGBMap[c].ID] = provincesRGBMap[c]
			}
		}
	}
	return nil
}

func findProvincesCenterPoints() {
	fmt.Printf("%s: Calculating provinces center point coordinates...\n", time.Since(startTime))
	for _, p := range provincesIDMap {
		p.CenterPoint = findCenterPoint(p.PixelCoords)
	}
}

func findCenterPoint(coords []image.Point) image.Point {
	// Fast centerpoint calculation.
	x := 0
	y := 0

	for _, c := range coords {
		x += c.X
		y += c.Y
	}

	return image.Point{int(math.Round(float64(x) / float64(len(coords)))), int(math.Round(float64(y) / float64(len(coords))))}

	// // Long largest rects centerpoint calculation.
	// l := math.MaxInt64
	// r := math.MinInt64
	// t := math.MaxInt64
	// b := math.MinInt64
	// for _, c := range coords {
	// 	if c.X < l {
	// 		l = c.X
	// 	}
	// 	if c.X > r {
	// 		r = c.X
	// 	}
	// 	if c.Y < t {
	// 		t = c.Y
	// 	}
	// 	if c.Y > b {
	// 		b = c.Y
	// 	}
	// }

	// maxRectSize := -1
	// var maxRect image.Rectangle
	// line := make([]int, r-l+1)
	// for y := t; y <= b; y++ {
	// 	i := 0
	// 	for x := l; x <= r; x++ {
	// 		if containsPoint(coords, image.Point{x, y}) {
	// 			line[i]++
	// 		} else {
	// 			line[i] = 0
	// 		}
	// 		i++
	// 	}
	// 	// fmt.Println(line)

	// 	rectSize, xStart, xEnd, yStart := findLargestRectangle(line)
	// 	if maxRectSize < rectSize {
	// 		maxRectSize = rectSize
	// 		maxRect.Min = image.Point{l + xStart, y - yStart + 1}
	// 		maxRect.Max = image.Point{l + xEnd - 1, y - 1}
	// 	}
	// }
	// // fmt.Println("> ", l, t, r, b, maxRectSize, maxRect, image.Point{int(math.Round(float64(maxRect.Min.X+maxRect.Max.X) / 2)), int(math.Round(float64(maxRect.Min.Y+maxRect.Max.Y) / 2))})

	// return image.Point{int(math.Round(float64(maxRect.Min.X+maxRect.Max.X) / 2)), int(math.Round(float64(maxRect.Min.Y+maxRect.Max.Y) / 2))}
}

func findAverageRadius(coords []image.Point, centerPoint image.Point) float64 {
	var radiusCenter float64 = 1.0
	for _, c := range coords {
		//Find cord distance to coordinate point
		rad := math.Sqrt(float64((c.X) ^ 2 + (c.Y) ^ 2))
		//multiply
		rad = math.Log(rad)
		radiusCenter += rad
		//fmt.Printf("radius of point: %v \n ", radiusCenter)
	}
	//Find geometric mean
	radiusCenter = math.Exp(radiusCenter / float64(len(coords)))
	//fmt.Printf("radius of state: %v \n Done State \n", radiusCenter)
	return radiusCenter
}

func findLargestRectangle(hist []int) (int, int, int, int) {
	var h, pos, tempH, tempPos int
	var xStart, xEnd, yStart int
	var hStack, posStack []int
	maxSize := -1
	tempSize := -1

	for pos = 0; pos < len(hist); pos++ {
		h = hist[pos]
		if len(hStack) == 0 || h > hStack[len(hStack)-1] {
			hStack = append(hStack, h)
			posStack = append(posStack, pos)
		} else if h < hStack[len(hStack)-1] {
			for len(hStack) > 0 && h < hStack[len(hStack)-1] {
				hStack, posStack, tempH, tempPos, tempSize = popStack(hStack, posStack, pos, maxSize)
				if maxSize < tempSize {
					maxSize = tempSize
					xStart = tempPos
					xEnd = pos
					yStart = tempH
				}
			}
			hStack = append(hStack, h)
			posStack = append(posStack, tempPos)
		}
	}
	return maxSize, xStart, xEnd, yStart
}

func popStack(hStack, posStack []int, pos, maxSize int) ([]int, []int, int, int, int) {
	tempH, hStack := hStack[len(hStack)-1], hStack[:len(hStack)-1]
	tempPos, posStack := posStack[len(posStack)-1], posStack[:len(posStack)-1]
	tempSize := tempH * (pos - tempPos)
	return hStack, posStack, tempH, tempPos, tempSize
}

func maxInt(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func parseStateFiles() error {
	fmt.Printf("%s: Parsing state files...\n", time.Since(startTime))
	stateFiles, err := filepath.Glob(filepath.FromSlash(statesPath) + string(os.PathSeparator) + "*.txt")
	if err != nil {
		return err
	}
	for _, s := range stateFiles {
		state, err := parseState(s)
		if errors.Is(err, errStateEmptyError) {
			fmt.Printf("State is empty, skipping %v \n", s)
		} else if err != nil {
			return err
		} else {
			if _, ok := statesMap[state.ID]; !ok {
				statesMap[state.ID] = &state
			} else {
				fmt.Printf("Duplicate state file found: %v\n", state.ID)
			}
		}
	}
	if readVanillaStates {
		fmt.Print("Reading vanilla histoy\n")
		stateFiles, err = filepath.Glob(filepath.FromSlash(hoi4Path+"/history/states") + string(os.PathSeparator) + "*.txt")
		if err != nil {
			return err
		}
		//Iterate through vanilla states
		for _, s := range stateFiles {
			state, err := parseState(s)
			if errors.Is(err, errStateEmptyError) {
				fmt.Printf("State is empty, skipping %v \n", s)
			} else if err != nil {
				return err
			} else {
				// if not in map, add this state to the map
				if _, ok := statesMap[state.ID]; !ok {
					fmt.Printf("Adding state from vanilla: %v\n", state.ID)
					statesMap[state.ID] = &state
				} else {
					fmt.Printf("Duplicate state file found in vanilla: %v\n", state.ID)
				}
			}
		}
	}
	return nil
}

var errStateEmptyError = errors.New("state empty")

func parseState(path string) (state State, err error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return state, err
	}
	s := strings.Replace(string(b), "\r\n", "\n", -1)
	s = rComment.ReplaceAllLiteralString(s, "")
	if strings.TrimSpace(s) == "" {
		return state, errStateEmptyError
	}
	submatch := rStateID.FindStringSubmatch(s)[1]
	state.ID, err = strconv.Atoi(submatch)
	if err != nil {
		return state, err
	}

	r := rStateName.FindStringSubmatch(s)
	if r != nil {
		state.Name = r[1]
	}

	r = rStateManpower.FindStringSubmatch(s)
	if r != nil {
		state.Manpower, err = strconv.Atoi(r[1])
		if err != nil {
			return state, err
		}

	}

	r = rStateInfrastructure.FindStringSubmatch(s)
	if r != nil {
		state.Infrastructure, err = strconv.Atoi(r[1])
		if err != nil {
			return state, err
		}

	}

	r = rStateImpassable.FindStringSubmatch(s)
	if r != nil {
		state.IsImpassable = true
	}

	state.Provinces = make(map[int]*Province)
	provinces := strings.Split(strings.TrimSpace(rSpace.ReplaceAllString(rStateProvinces.FindStringSubmatch(s)[1], " ")), " ")
	//fmt.Printf("Provinces: %v", provinces)
	for _, p := range provinces {
		if len(p) > 0 {
			pID, err := strconv.Atoi(p)

			if err != nil {
				fmt.Printf("Error in state: %v", state)
				return state, err
			}
			state.Provinces[pID] = provincesIDMap[pID]
		}
	}
	state.Continent = -1
	state.PixelCoordsMap = make(map[image.Point]bool)
	state.DistanceTo = make(map[int]int)
	state.AdjacentTo = make(map[int]*State)
	state.ConnectedTo = make(map[int]*State)
	state.ImpassableTo = make(map[int]*State)
	return state, nil
}

func parseStatesProvinces() {
	fmt.Printf("%s: Parsing provinces in each state...\n", time.Since(startTime))
	for _, s1 := range statesMap {
		for _, p1 := range s1.Provinces {
			// All provinces in a state should have the same continent number.
			// Save the first province continent as states continent.
			if s1.Continent == -1 {
				s1.Continent = p1.Continent
			}

			// If there is at least one coastal province in a state, mark state as coastal.
			if p1.IsCoastal {
				s1.IsCoastal = true
			}

			// Fill in each states pixel coordinates.
			for _, pc := range p1.PixelCoords {
				s1.PixelCoordsMap[pc] = true
				s1.PixelCoords = append(s1.PixelCoords, pc)
			}

			// Fill up adjacentTo and connectedTo fields in all states
			// based on the provinces in those states
			for _, s2 := range statesMap {
				for _, p2 := range s2.Provinces {
					for _, a1 := range p1.AdjacentTo {
						if a1.ID == p2.ID && s1.ID != s2.ID {
							s1.AdjacentTo[s2.ID] = s2
						}
					}
					for _, c1 := range p1.ConnectedTo {
						if c1.ID == p2.ID && s1.ID != s2.ID {
							s1.ConnectedTo[s2.ID] = s2
						}
					}
				}
			}

			// Add state to the province.
			p1.State = s1
		}
	}

	for _, s1 := range statesMap {
		// Find the center point of the state.
		// fmt.Printf("%s: Calculating states center point coordinates...\n", time.Since(startTime))
		s1.CenterPoint = findCenterPoint(s1.PixelCoords)
		s1.StateSize = findAverageRadius(s1.PixelCoords, s1.CenterPoint)
		// If state has provinces with non-empty impassableTo field.
		// Check if all provinces adjacent to another state are impassable to it.
		// If that's the case, then add this state to impassableTo filed of the first sate.
		impassableProvincesCount := 0
		for _, p1 := range s1.Provinces {
			if len(p1.ImpassableTo) > 0 {
				impassableProvincesCount++
			}
		}
		if impassableProvincesCount > 0 {
			for _, s2 := range s1.AdjacentTo {
				adjacentProvinces := make(map[int]struct{})
				adjacentProvincesCount := 0
				impassableProvincesCount = 0
				for _, p1 := range s1.Provinces {
					for _, ap1 := range p1.AdjacentTo {
						for _, p2 := range s2.Provinces {
							if ap1.ID == p2.ID {
								adjacentProvinces[ap1.ID] = struct{}{}
								adjacentProvincesCount++
							}
						}
					}
					for _, i1 := range p1.ImpassableTo {
						if _, ok := adjacentProvinces[i1.ID]; ok {
							impassableProvincesCount++
						}
					}
				}
				if impassableProvincesCount > 0 && impassableProvincesCount == adjacentProvincesCount {
					s1.ImpassableTo[s2.ID] = s2
				}
			}
		}
	}
}

func parseStatesDistanceToOtherStates() {
	fmt.Printf("%s: Calculating distance between each state...\n", time.Since(startTime))
	for _, s1 := range statesMap {
		for _, s2 := range statesMap {
			s1.DistanceTo[s2.ID] = distance(s1.CenterPoint, s2.CenterPoint)
		}
	}
}

// Distance returns rounded distance between two coordinates.
func distance(c1, c2 image.Point) int {
	return int(math.Round(math.Sqrt(math.Pow(float64(c2.X-c1.X), 2)+math.Pow(float64(c2.Y-c1.Y), 2)) * mapScalePixelToKm))
}

func parseStrategicRegionFiles() error {
	fmt.Printf("%s: Parsing strategic region files...\n", time.Since(startTime))
	strategicRegionFiles, err := filepath.Glob(filepath.FromSlash(strategicRegionPath) + string(os.PathSeparator) + "*.txt")
	if err != nil {
		return err
	}
	for _, r := range strategicRegionFiles {
		strategicRegion, err := parseStrategicRegion(r)
		if err != nil {
			fmt.Printf("Error in Region: %v", strategicRegion)
			return err
		}
		strategicRegionMap[strategicRegion.ID] = &strategicRegion
	}
	return nil
}

func parseStrategicRegion(path string) (strategicRegion StrategicRegion, err error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return strategicRegion, err
	}
	s := strings.Replace(string(b), "\r\n", "\n", -1)

	strategicRegion.ID, err = strconv.Atoi(rStateID.FindStringSubmatch(s)[1])
	if err != nil {
		return strategicRegion, err
	}

	r := rStateName.FindStringSubmatch(s)
	if r != nil {
		strategicRegion.Name = r[1]
	}

	strategicRegion.Provinces = make(map[int]*Province)
	provinces := strings.Split(strings.TrimSpace(rSpace.ReplaceAllString(rStateProvinces.FindStringSubmatch(s)[1], " ")), " ")
	//fmt.Printf("Strat Provinces for %v: %v", strategicRegion.ID, provinces)
	for _, p := range provinces {
		if len(p) > 0 {
			pID, err := strconv.Atoi(p)
			if err != nil {
				fmt.Printf("%v had an error at %v", strategicRegion.ID, p)
				return strategicRegion, err
			}
			strategicRegion.Provinces[pID] = provincesIDMap[pID]
		}
	}

	strategicRegion.PixelCoordsMap = make(map[image.Point]bool)

	return strategicRegion, nil
}

func parseStrategicRegionsProvinces() {
	fmt.Printf("%s: Parsing provinces in each strategic region...\n", time.Since(startTime))
	for _, r := range strategicRegionMap {
		//fmt.Printf("Finished: %v \n", r.ID)
		//fmt.Printf("Provinces in %v: %v \n", r.ID, r.Provinces)
		for _, p := range r.Provinces {
			// Fill in each strategic regions pixel coordinates.
			for _, pc := range p.PixelCoords {
				r.PixelCoordsMap[pc] = true
				r.PixelCoords = append(r.PixelCoords, pc)
			}

			// Add strategic region to the province.
			p.StrategicRegion = r
		}
		// // Find the center point of the strategic region.
		// // fmt.Printf("%s: Calculating strategic regions center point coordinates...\n", time.Since(startTime))
		// r.CenterPoint = findCenterPoint(r.PixelCoords)
	}
}

func saveGeoData() error {
	fmt.Printf("%s: Writing the output file...\n", time.Since(startTime))
	// Create new file.
	f, err := os.Create("hoi4geoparser_data.txt")
	if err != nil {
		return err
	}
	defer f.Close()

	// Write on_actions header into the output file.
	_, err = f.WriteString("# Autogenerated by hoi4geoparser. Do not mess with the data.\n# evil_c0okie (https://github.com/malashin/hoi4geoparser)\n\non_actions = {\n\ton_startup = {\n\t\teffect = {\n")
	if err != nil {
		return err
	}

	// Sort the state ids.
	statesIDs := sortedKeySliceFromStateMap(statesMap)
	// Iterate over all states in ID sorted order.
	for _, sID := range statesIDs {
		// if len(statesMap[sID].ConnectedTo) == 0 && len(statesMap[sID].ImpassableTo) == 0 {
		// 	continue
		// }

		if len(statesMap[sID].ImpassableTo) == 0 && !statesMap[sID].IsImpassable {
			continue
		}

		// Write the state id into the output file.
		_, err = f.WriteString("\t\t\t" + strconv.Itoa(sID) + " = {\n")
		if err != nil {
			return err
		}

		// if len(statesMap[sID].ConnectedTo) > 0 {
		// 	// Sort the map.
		// 	statesConnectedToIDs := sortedKeySliceFromStateMap(statesMap[sID].ConnectedTo)
		// 	// Iterate over all states from ConnectedTo map in ID sorted order.
		// 	for _, cID := range statesConnectedToIDs {
		// 		// Write the connected_to@STATE variables.
		// 		_, err = f.WriteString("\t\t\t\tset_variable = { connected_to@" + strconv.Itoa(cID) + " = 1 }\n")
		// 		if err != nil {
		// 			return err
		// 		}
		// 	}
		// }

		if len(statesMap[sID].ImpassableTo) > 0 {
			// Sort the map.
			statesImpassableToIDs := sortedKeySliceFromStateMap(statesMap[sID].ImpassableTo)
			// Iterate over all states from ImpassableTo map in ID sorted order.
			for _, aID := range statesImpassableToIDs {
				// Write the impassable_to@STATE variables.
				_, err = f.WriteString("\t\t\t\tset_variable = { impassable_to" + strconv.Itoa(aID) + " = 1 }\n")
				if err != nil {
					return err
				}
			}
		}

		if statesMap[sID].IsImpassable {
			// Write the is_impassable state flag.
			_, err = f.WriteString("\t\t\t\tset_state_flag = is_impassable\n")
			if err != nil {
				return err
			}
		}

		// // Sort the map.
		// statesDistanceToIDs := sortedKeySliceFromIntMap(statesMap[sID].DistanceTo)
		// // Iterate over all states from DistanceTO map in ID sorted order.
		// for _, dID := range statesDistanceToIDs {
		// 	// Write the distance_to@STATE variables.
		// 	_, err = f.WriteString("\t\t\t\tset_variable = { distance_to@" + strconv.Itoa(dID) + " = " + strconv.Itoa(statesMap[sID].DistanceTo[dID]) + " }\n")
		// 	if err != nil {
		// 		return err
		// 	}
		// }

		// Write the state closing brackets into the output file.
		_, err = f.WriteString("\t\t\t}\n")
		if err != nil {
			return err
		}
	}

	// Write the on_startup and effect closing brackets into the output file.
	_, err = f.WriteString("\t\t}\n\t}\n}\n")
	if err != nil {
		return err
	}

	return nil
}

func sortedKeySliceFromStateMap(m map[int]*State) (slice []int) {
	for k := range m {
		slice = append(slice, k)
	}
	sort.Ints(slice)
	return slice
}

func sortedKeySliceFromIntMap(m map[int]int) (slice []int) {
	for k := range m {
		slice = append(slice, k)
	}
	sort.Ints(slice)
	return slice
}

func generateRandomLightColor() color.RGBA {
	max := 255
	min := 128
	c := color.RGBA{uint8(rand.Intn(max-min) + min), uint8(rand.Intn(max-min) + min), uint8(rand.Intn(max-min) + min), 255}
	// if isColorClose(c, waterColor) {
	// 	c = generateRandomLightColor()
	// }
	return c
}

func isColorClose(a color.RGBA, b color.RGBA) bool {
	d := math.Sqrt(2*math.Exp2(float64(b.R-a.R)) + 4*math.Exp2(float64(b.G-a.G)) + 3*math.Exp2(float64(b.B-a.B)))
	// fmt.Printf("%v %v  |  %e  %f  %v\n", a, b, d, d, d < 10000000000000000000000000000000000)
	return d < 10000000000000000000000000000000000
}

func generateRandomStateColor(s *State, i int) {
	b := false
	col := generateRandomLightColor()

	for _, a := range s.AdjacentTo {
		// fmt.Println(s.ID, col, a.ID, a.RenderColor)
		if (a.RenderColor != color.RGBA{0, 0, 0, 0}) && (isColorClose(col, a.RenderColor)) {
			b = true
			continue
		}
	}

	if b && (i < 500) {
		generateRandomStateColor(s, i+1)
	}

	s.RenderColor = col
}

func addLabel(img *image.RGBA, c *freetype.Context, x, y int, size float64, label string) error {
	pt := freetype.Pt(x, y)
	if _, err := c.DrawString(label, pt); err != nil {
		return err
	}
	return nil
}

func generateSateMap() error {
	fmt.Printf("%s: Generating state map...\n", time.Since(startTime))

	// Create empty image and fill it with blue color (water).
	img := image.NewRGBA(provincesImageSize)
	draw.Draw(img, img.Bounds(), &image.Uniform{waterColor}, image.ZP, draw.Src)

	// // Draw state shapes.
	// for _, s := range statesMap {
	// 	generateRandomStateColor(s, 0)
	// 	for _, p := range s.PixelCoords {
	// 		img.Set(p.X, p.Y, s.RenderColor)
	// 	}
	// }

	// Draw land province shapes.
	fillCol := color.RGBA{255, 255, 255, 255}
	for _, prov := range provincesIDMap {
		if prov.Type == "land" {
			for _, p := range prov.PixelCoords {
				img.Set(p.X, p.Y, fillCol)
			}
		}
	}

	// Draw state borders.
	stateBorderColor := color.RGBA{158, 158, 158, 255}
	for _, s := range statesMap {
		for _, p := range s.PixelCoords {
			_, exists := s.PixelCoordsMap[image.Point{p.X + 1, p.Y}]
			if !exists {
				img.Set(p.X+1, p.Y, stateBorderColor)
			}
			_, exists = s.PixelCoordsMap[image.Point{p.X, p.Y + 1}]
			if !exists {
				img.Set(p.X, p.Y+1, stateBorderColor)
			}
			_, exists = s.PixelCoordsMap[image.Point{p.X - 1, p.Y}]
			if !exists {
				img.Set(p.X, p.Y, stateBorderColor)
			}
			_, exists = s.PixelCoordsMap[image.Point{p.X, p.Y - 1}]
			if !exists {
				img.Set(p.X, p.Y, stateBorderColor)
			}
		}
	}

	// Draw strategic region borders.
	strategicRegionBorderColor := color.RGBA{158, 158, 158, 255}
	for _, r := range strategicRegionMap {
		for _, p := range r.PixelCoords {
			_, exists := r.PixelCoordsMap[image.Point{p.X + 1, p.Y}]
			if !exists {
				img.Set(p.X+1, p.Y, strategicRegionBorderColor)
			}
			_, exists = r.PixelCoordsMap[image.Point{p.X, p.Y + 1}]
			if !exists {
				img.Set(p.X, p.Y+1, strategicRegionBorderColor)
			}
			_, exists = r.PixelCoordsMap[image.Point{p.X - 1, p.Y}]
			if !exists {
				img.Set(p.X, p.Y, strategicRegionBorderColor)
			}
			_, exists = r.PixelCoordsMap[image.Point{p.X, p.Y - 1}]
			if !exists {
				img.Set(p.X, p.Y, strategicRegionBorderColor)
			}
		}
	}

	// Draw lake province shapes over the land.
	for _, prov := range provincesIDMap {
		if prov.Type == "lake" {
			for _, p := range prov.PixelCoords {
				img.Set(p.X, p.Y, waterColor)
			}
		}
	}

	// Save image as PNG.
	out, err := os.Create("./state_map.png")
	if err != nil {
		return err
	}
	err = png.Encode(out, img)
	if err != nil {
		return err
	}
	fmt.Printf("%s: Saved 'state_map.png'\n", time.Since(startTime))
	return nil
}

func generateColoredSateMap() error {
	fmt.Printf("%s: Generating colored state map...\n", time.Since(startTime))

	// Create empty image and fill it with blue color (water).
	img := image.NewRGBA(provincesImageSize)
	draw.Draw(img, img.Bounds(), &image.Uniform{waterColor}, image.ZP, draw.Src)

	// Draw state shapes.
	for _, s := range statesMap {
		generateRandomStateColor(s, 0)
		for _, p := range s.PixelCoords {
			img.Set(p.X, p.Y, s.RenderColor)
		}
	}

	// Draw lake province shapes over the land.
	for _, prov := range provincesIDMap {
		if prov.Type == "lake" {
			for _, p := range prov.PixelCoords {
				img.Set(p.X, p.Y, waterColor)
			}
		}
	}

	// Save image as PNG.
	out, err := os.Create("./state_map_colored.png")
	if err != nil {
		return err
	}
	err = png.Encode(out, img)
	if err != nil {
		return err
	}
	fmt.Printf("%s: Saved 'state_map_colored.png'\n", time.Since(startTime))
	return nil
}

func generateSateIDMap() error {
	fmt.Printf("%s: Generating state ID map...\n", time.Since(startTime))

	// Create empty image and fill it with blue color (water).
	img := image.NewRGBA(provincesImageSize)
	draw.Draw(img, img.Bounds(), &image.Uniform{waterColor}, image.ZP, draw.Src)

	// Draw land province shapes.
	fillCol := color.RGBA{255, 255, 255, 255}
	for _, prov := range provincesIDMap {
		if prov.Type == "land" {
			for _, p := range prov.PixelCoords {
				img.Set(p.X, p.Y, fillCol)
			}
		}
	}

	// Draw state borders.
	stateBorderColor := color.RGBA{158, 158, 158, 255}
	for _, s := range statesMap {
		for _, p := range s.PixelCoords {
			_, exists := s.PixelCoordsMap[image.Point{p.X + 1, p.Y}]
			if !exists {
				img.Set(p.X+1, p.Y, stateBorderColor)
			}
			_, exists = s.PixelCoordsMap[image.Point{p.X, p.Y + 1}]
			if !exists {
				img.Set(p.X, p.Y+1, stateBorderColor)
			}
			_, exists = s.PixelCoordsMap[image.Point{p.X - 1, p.Y}]
			if !exists {
				img.Set(p.X, p.Y, stateBorderColor)
			}
			_, exists = s.PixelCoordsMap[image.Point{p.X, p.Y - 1}]
			if !exists {
				img.Set(p.X, p.Y, stateBorderColor)
			}
		}
	}

	// Draw strategic region borders.
	strategicRegionBorderColor := color.RGBA{158, 158, 158, 255}
	for _, r := range strategicRegionMap {
		for _, p := range r.PixelCoords {
			_, exists := r.PixelCoordsMap[image.Point{p.X + 1, p.Y}]
			if !exists {
				img.Set(p.X+1, p.Y, strategicRegionBorderColor)
			}
			_, exists = r.PixelCoordsMap[image.Point{p.X, p.Y + 1}]
			if !exists {
				img.Set(p.X, p.Y+1, strategicRegionBorderColor)
			}
			_, exists = r.PixelCoordsMap[image.Point{p.X - 1, p.Y}]
			if !exists {
				img.Set(p.X, p.Y, strategicRegionBorderColor)
			}
			_, exists = r.PixelCoordsMap[image.Point{p.X, p.Y - 1}]
			if !exists {
				img.Set(p.X, p.Y, strategicRegionBorderColor)
			}
		}
	}

	// Init font.
	c, err := initFont(img)
	if err != nil {
		return err
	}

	//Draw state IDs.
	for _, s := range statesMap {
		n := strconv.FormatInt(int64(s.ID), 10)
		offset := 0
		if n != "" {
			offset = (len(n)*charWidth - strings.Count(n, "1") + len(n) - 1) / 2
		}
		err := addLabel(img, c, s.CenterPoint.X-offset, s.CenterPoint.Y+charHeight/2+1, 10.0, n)
		if err != nil {
			return err
		}
	}

	// Save image as PNG.
	out, err := os.Create("./state_map_with_ids.png")
	if err != nil {
		return err
	}
	err = png.Encode(out, img)
	if err != nil {
		return err
	}
	fmt.Printf("%s: Saved 'state_map_with_ids.png'\n", time.Since(startTime))
	return nil
}

func initFont(img *image.RGBA) (*freetype.Context, error) {
	// Read the font data.
	f, err := freetype.ParseFont(fontBytes)
	if err != nil {
		return nil, err
	}

	// Initialize font's context.
	fg := image.Black
	c := freetype.NewContext()
	c.SetDPI(72.0)
	c.SetFont(f)
	c.SetFontSize(10.0)
	c.SetClip(img.Bounds())
	c.SetDst(img)
	c.SetSrc(fg)
	c.SetHinting(font.HintingNone)
	return c, nil
}

func generateProvinceMap() error {
	fmt.Printf("%s: Generating province map...\n", time.Since(startTime))

	// Create empty image and fill it with blue color (water).
	img := image.NewRGBA(provincesImageSize)
	draw.Draw(img, img.Bounds(), &image.Uniform{waterColor}, image.ZP, draw.Src)

	// Draw land province shapes.
	fillCol := color.RGBA{255, 255, 255, 255}
	for _, prov := range provincesIDMap {
		if prov.Type == "land" {
			for _, p := range prov.PixelCoords {
				img.Set(p.X, p.Y, fillCol)
			}
		}
	}

	// Draw province borders.
	provinceBorderColor := color.RGBA{158, 158, 158, 255}
	for _, prov := range provincesIDMap {
		for _, p := range prov.PixelCoords {
			_, exists := prov.PixelCoordsMap[image.Point{p.X + 1, p.Y}]
			if !exists {
				img.Set(p.X+1, p.Y, provinceBorderColor)
			}
			_, exists = prov.PixelCoordsMap[image.Point{p.X, p.Y + 1}]
			if !exists {
				img.Set(p.X, p.Y+1, provinceBorderColor)
			}
		}
	}

	// Draw state borders.
	stateBorderColor := color.RGBA{255, 0, 0, 255}
	for _, s := range statesMap {
		for _, p := range s.PixelCoords {
			_, exists := s.PixelCoordsMap[image.Point{p.X + 1, p.Y}]
			if !exists {
				img.Set(p.X+1, p.Y, stateBorderColor)
			}
			_, exists = s.PixelCoordsMap[image.Point{p.X, p.Y + 1}]
			if !exists {
				img.Set(p.X, p.Y+1, stateBorderColor)
			}
			_, exists = s.PixelCoordsMap[image.Point{p.X - 1, p.Y}]
			if !exists {
				img.Set(p.X, p.Y, stateBorderColor)
			}
			_, exists = s.PixelCoordsMap[image.Point{p.X, p.Y - 1}]
			if !exists {
				img.Set(p.X, p.Y, stateBorderColor)
			}
		}
	}

	// Draw strategic region borders.
	strategicRegionBorderColor := color.RGBA{255, 0, 0, 255}
	for _, r := range strategicRegionMap {
		for _, p := range r.PixelCoords {
			_, exists := r.PixelCoordsMap[image.Point{p.X + 1, p.Y}]
			if !exists {
				img.Set(p.X+1, p.Y, strategicRegionBorderColor)
			}
			_, exists = r.PixelCoordsMap[image.Point{p.X, p.Y + 1}]
			if !exists {
				img.Set(p.X, p.Y+1, strategicRegionBorderColor)
			}
			_, exists = r.PixelCoordsMap[image.Point{p.X - 1, p.Y}]
			if !exists {
				img.Set(p.X, p.Y, strategicRegionBorderColor)
			}
			_, exists = r.PixelCoordsMap[image.Point{p.X, p.Y - 1}]
			if !exists {
				img.Set(p.X, p.Y, strategicRegionBorderColor)
			}
		}
	}

	// Save image as PNG.
	out, err := os.Create("./province_map.png")
	if err != nil {
		return err
	}
	err = png.Encode(out, img)
	if err != nil {
		return err
	}
	fmt.Printf("%s: Saved 'province_map.png'\n", time.Since(startTime))

	return nil
}

func generateProvinceIDMap() error {
	fmt.Printf("%s: Generating province ID map...\n", time.Since(startTime))

	// Create empty image and fill it with blue color (water).
	img := image.NewRGBA(provincesImageSize)
	draw.Draw(img, img.Bounds(), &image.Uniform{waterColor}, image.ZP, draw.Src)

	// Draw land province shapes.
	fillCol := color.RGBA{255, 255, 255, 255}
	for _, prov := range provincesIDMap {
		if prov.Type == "land" {
			for _, p := range prov.PixelCoords {
				img.Set(p.X, p.Y, fillCol)
			}
		}
	}

	// Scale image up.
	dst := image.NewRGBA(image.Rect(0, 0, img.Bounds().Max.X*4, img.Bounds().Max.Y*4))
	draw.NearestNeighbor.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	img = dst

	// Draw province borders.
	provinceBorderColor := color.RGBA{158, 158, 158, 255}
	for _, prov := range provincesIDMap {
		for _, p := range prov.PixelCoords {
			_, exists := prov.PixelCoordsMap[image.Point{p.X + 1, p.Y}]
			if !exists {
				img.Set(p.X*4+3, p.Y*4, provinceBorderColor)
				img.Set(p.X*4+3, p.Y*4+1, provinceBorderColor)
				img.Set(p.X*4+3, p.Y*4+2, provinceBorderColor)
				img.Set(p.X*4+3, p.Y*4+3, provinceBorderColor)
			}
			_, exists = prov.PixelCoordsMap[image.Point{p.X, p.Y + 1}]
			if !exists {
				img.Set(p.X*4, p.Y*4+3, provinceBorderColor)
				img.Set(p.X*4+1, p.Y*4+3, provinceBorderColor)
				img.Set(p.X*4+2, p.Y*4+3, provinceBorderColor)
				img.Set(p.X*4+3, p.Y*4+3, provinceBorderColor)
			}
		}
	}

	// Draw state borders.
	stateBorderColor := color.RGBA{255, 0, 0, 255}
	for _, s := range statesMap {
		for _, p := range s.PixelCoords {
			_, exists := s.PixelCoordsMap[image.Point{p.X + 1, p.Y}]
			if !exists {
				img.Set(p.X*4+3, p.Y*4, stateBorderColor)
				img.Set(p.X*4+3, p.Y*4+1, stateBorderColor)
				img.Set(p.X*4+3, p.Y*4+2, stateBorderColor)
				img.Set(p.X*4+3, p.Y*4+3, stateBorderColor)
			}
			_, exists = s.PixelCoordsMap[image.Point{p.X, p.Y + 1}]
			if !exists {
				img.Set(p.X*4, p.Y*4+3, stateBorderColor)
				img.Set(p.X*4+1, p.Y*4+3, stateBorderColor)
				img.Set(p.X*4+2, p.Y*4+3, stateBorderColor)
				img.Set(p.X*4+3, p.Y*4+3, stateBorderColor)
			}
			_, exists = s.PixelCoordsMap[image.Point{p.X - 1, p.Y}]
			if !exists {
				img.Set(p.X*4-1, p.Y*4, stateBorderColor)
				img.Set(p.X*4-1, p.Y*4+1, stateBorderColor)
				img.Set(p.X*4-1, p.Y*4+2, stateBorderColor)
				img.Set(p.X*4-1, p.Y*4+3, stateBorderColor)
			}
			_, exists = s.PixelCoordsMap[image.Point{p.X, p.Y - 1}]
			if !exists {
				img.Set(p.X*4, p.Y*4-1, stateBorderColor)
				img.Set(p.X*4+1, p.Y*4-1, stateBorderColor)
				img.Set(p.X*4+2, p.Y*4-1, stateBorderColor)
				img.Set(p.X*4+3, p.Y*4-1, stateBorderColor)
			}
		}
	}

	// Init font.
	c, err := initFont(img)
	if err != nil {
		return err
	}

	//Draw province IDs.
	for _, p := range provincesIDMap {
		n := strconv.FormatInt(int64(p.ID), 10)
		offset := 0
		if n != "" {
			offset = (len(n)*charWidth - strings.Count(n, "1") + len(n) - 1) / 2
		}
		err := addLabel(img, c, p.CenterPoint.X*4-offset, p.CenterPoint.Y*4+charHeight/2+1, 10.0, n)
		if err != nil {
			return err
		}
	}

	// Save image as PNG.
	out, err := os.Create("./province_id_map.png")
	if err != nil {
		return err
	}
	err = png.Encode(out, img)
	if err != nil {
		return err
	}
	fmt.Printf("%s: Saved 'province_id_map.png'\n", time.Since(startTime))

	return nil
}

func generateManpowerMap() error {
	fmt.Printf("%s: Generating manpower map...\n", time.Since(startTime))

	// Create empty image and fill it with blue color (water).
	img := image.NewRGBA(provincesImageSize)
	draw.Draw(img, img.Bounds(), &image.Uniform{waterColor}, image.ZP, draw.Src)

	// Find highest manpower value in a state.
	// mpMin := 200000 // Base game
	// mpMin := 10000 // EaW
	mpMin := 1000 // OWB
	mpMax := 0
	for _, s := range statesMap {
		if s.Manpower > mpMax {
			mpMax = s.Manpower
		}
	}
	logMin := math.Log10(float64(mpMin))
	logMax := math.Log10(float64(mpMax))
	logRange := logMax - logMin

	// Draw state shapes.
	colorLow := color.RGBA{255, 64, 64, 255}
	colorMid := color.RGBA{255, 255, 64, 255}
	colorHigh := color.RGBA{64, 255, 64, 255}
	gradient := []color.RGBA{colorLow, colorMid, colorHigh}

	for _, s := range statesMap {
		// mp := float64(s.Manpower) / float64(mpMax)
		mp := linearToLog(math.Max(float64(s.Manpower), float64(mpMin)), logMin, logRange)
		fillCol := colorFromGradient(mp, gradient)
		for _, p := range s.PixelCoords {
			img.Set(p.X, p.Y, fillCol)
		}
	}

	// Draw lake province shapes over the land.
	for _, prov := range provincesIDMap {
		if prov.Type == "lake" {
			for _, p := range prov.PixelCoords {
				img.Set(p.X, p.Y, waterColor)
			}
		}
	}

	// Draw state borders.
	stateBorderColor := color.RGBA{158, 158, 158, 255}
	for _, s := range statesMap {
		for _, p := range s.PixelCoords {
			_, exists := s.PixelCoordsMap[image.Point{p.X + 1, p.Y}]
			if !exists {
				img.Set(p.X+1, p.Y, stateBorderColor)
			}
			_, exists = s.PixelCoordsMap[image.Point{p.X, p.Y + 1}]
			if !exists {
				img.Set(p.X, p.Y+1, stateBorderColor)
			}
			_, exists = s.PixelCoordsMap[image.Point{p.X - 1, p.Y}]
			if !exists {
				img.Set(p.X, p.Y, stateBorderColor)
			}
			_, exists = s.PixelCoordsMap[image.Point{p.X, p.Y - 1}]
			if !exists {
				img.Set(p.X, p.Y, stateBorderColor)
			}
		}
	}

	// Init font.
	c, err := initFont(img)
	if err != nil {
		return err
	}

	//Draw state manpower values.
	for _, s := range statesMap {
		n := intToString(s.Manpower)
		offset := 0
		if n != "" {
			offset = (len(n)*charWidth - strings.Count(n, "1") + len(n) - 1) / 2
		}
		err := addLabel(img, c, s.CenterPoint.X-offset, s.CenterPoint.Y+charHeight/2+1, 10.0, n)
		if err != nil {
			return err
		}
	}

	// Save image as PNG.
	out, err := os.Create("./manpower_map.png")
	if err != nil {
		return err
	}
	err = png.Encode(out, img)
	if err != nil {
		return err
	}
	fmt.Printf("%s: Saved 'manpower_map.png'\n", time.Since(startTime))

	return nil
}

func linearToLog(n, min, r float64) float64 {
	return (math.Log10(n) - min) / r
}

func colorFromGradient(a float64, gradient []color.RGBA) color.RGBA {
	b := 100 / float64(len(gradient)-1) / 100
	colorLow := gradient[int(math.Floor(a/b))]
	colorHigh := gradient[int(math.Ceil(a/b))]

	return color.RGBA{
		uint8(math.Min(math.Max(float64(colorLow.R)+(float64(colorHigh.R)-float64(colorLow.R))*a, 0), 255)),
		uint8(math.Min(math.Max(float64(colorLow.G)+(float64(colorHigh.G)-float64(colorLow.G))*a, 0), 255)),
		uint8(math.Min(math.Max(float64(colorLow.B)+(float64(colorHigh.B)-float64(colorLow.B))*a, 0), 255)),
		uint8(math.Min(math.Max(float64(colorLow.A)+(float64(colorHigh.A)-float64(colorLow.A))*a, 0), 255))}
}

func intToString(n int) string {
	if n < 1000 {
		return strconv.Itoa(n)
	}
	exp := math.Floor(math.Log(float64(n)) / math.Log(1000))
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(float64(n)/math.Pow(1000, exp), 'f', 1, 64), "0"), ".") + string("kMGTPE"[int(exp-1)])
}

func generateSeaProvinceMap() error {
	fmt.Printf("%s: Generating sea province map...\n", time.Since(startTime))

	// Create empty image and fill it with blue color (water).
	img := image.NewRGBA(provincesImageSize)
	draw.Draw(img, img.Bounds(), &image.Uniform{waterColor}, image.ZP, draw.Src)

	// Draw sea provinces.
	for _, prov := range provincesIDMap {
		if (prov.Type == "sea") || (prov.Type == "lake") {
			fillCol := generateRandomLightColor()
			for _, p := range prov.PixelCoords {
				img.Set(p.X, p.Y, fillCol)
			}
		}
	}

	// Save image as PNG.
	out, err := os.Create("./sea_province_map.png")
	if err != nil {
		return err
	}
	err = png.Encode(out, img)
	if err != nil {
		return err
	}
	fmt.Printf("%s: Saved 'sea_province_map.png'\n", time.Since(startTime))

	return nil
}

func generateProvinceBasedTerrainMap() error {
	fmt.Printf("%s: Generating province-based terrain map...\n", time.Since(startTime))

	terrainFile, err := os.Open(filepath.FromSlash(terrainPath))
	if err != nil {
		return err
	}
	defer terrainFile.Close()
	terrainImage, err := bmp.Decode(terrainFile)
	if err != nil {
		return err
	}

	img := image.NewRGBA(provincesImageSize)
	draw.Draw(img, img.Bounds(), terrainImage, terrainImage.Bounds().Min, draw.Src)

	for _, p := range provincesIDMap {
		terrainColors := make(map[color.RGBA]int)
		if p.Type == "land" {
			for _, pc := range p.PixelCoords {
				terrainColors[terrainImage.At(pc.X, pc.Y).(color.RGBA)]++
			}

			max := 0
			var terrainColor color.RGBA
			for c, i := range terrainColors {
				if i > max {
					max = i
					terrainColor = c
				}
			}

			for _, pc := range p.PixelCoords {
				img.Set(pc.X, pc.Y, color.RGBA{terrainColor.R, terrainColor.G, terrainColor.B, terrainColor.A})
			}
		}
	}

	// Save image as PNG.
	out, err := os.Create("./province_based_terrain.png")
	if err != nil {
		return err
	}
	err = png.Encode(out, img)
	if err != nil {
		return err
	}
	fmt.Printf("%s: Saved 'province_based_terrain.png'\n", time.Since(startTime))

	return nil
}

func generateProvinceBasedHeightmapThresholdMap() error {
	fmt.Printf("%s: Generating province-based heightmap threshold map...\n", time.Since(startTime))

	heightmapFile, err := os.Open(filepath.FromSlash(heightmapPath))
	if err != nil {
		return err
	}
	defer heightmapFile.Close()
	heightmapImage, err := bmp.Decode(heightmapFile)
	if err != nil {
		return err
	}

	img := image.NewRGBA(provincesImageSize)

	for _, p := range provincesIDMap {
		heightmapColors := make(map[color.RGBA]int)
		if p.Type == "land" {
			for _, pc := range p.PixelCoords {
				heightmapColors[heightmapImage.At(pc.X, pc.Y).(color.RGBA)]++
			}

			// Find dominant color in the province.
			max := 0
			var heightmapColor color.RGBA
			for c, i := range heightmapColors {
				if i > max {
					max = i
					heightmapColor = c
				}
			}

			// Color every province higher then that value pink.
			if heightmapColor.R > 222 {
				for _, pc := range p.PixelCoords {
					img.Set(pc.X, pc.Y, color.RGBA{255, 0, 255, 255})
				}
			} else {
				for _, pc := range p.PixelCoords {
					img.Set(pc.X, pc.Y, color.RGBA{heightmapColor.R, heightmapColor.G, heightmapColor.B, heightmapColor.A})
				}
			}

			var dark uint8 = 255
			var bright uint8
			// Find the bightest and darkest colors.
			for c := range heightmapColors {
				if c.R > bright {
					bright = c.R
				}
				if c.R < dark {
					dark = c.R
				}
			}
			if bright-dark > 100 {
				for _, pc := range p.PixelCoords {
					img.Set(pc.X, pc.Y, color.RGBA{255, 255, 0, 255})
				}
			}
		}
	}

	// Save image as PNG.
	out, err := os.Create("./province_based_heightmap_threshold.png")
	if err != nil {
		return err
	}
	err = png.Encode(out, img)
	if err != nil {
		return err
	}
	fmt.Printf("%s: Saved 'province_based_heightmap_threshold.png'\n", time.Since(startTime))

	return nil
}

func generateInfrastructureMap() error {
	fmt.Printf("%s: Generating infrastructure map...\n", time.Since(startTime))

	// Create empty image and fill it with blue color (water).
	img := image.NewRGBA(provincesImageSize)
	draw.Draw(img, img.Bounds(), &image.Uniform{waterColor}, image.ZP, draw.Src)

	// Find highest infrastructure value.
	iMax := 0
	for _, s := range statesMap {
		if s.Infrastructure > iMax {
			iMax = s.Infrastructure
		}
	}

	// Draw state shapes.
	colorLow := color.RGBA{255, 64, 64, 255}
	colorMid := color.RGBA{255, 255, 64, 255}
	colorHigh := color.RGBA{64, 255, 64, 255}
	gradient := []color.RGBA{colorLow, colorMid, colorHigh}

	for _, s := range statesMap {
		i := 100 / float64(iMax) * float64(s.Infrastructure) / 100
		fillCol := colorFromGradient(i, gradient)
		for _, p := range s.PixelCoords {
			img.Set(p.X, p.Y, fillCol)
		}
	}

	// Draw lake province shapes over the land.
	for _, prov := range provincesIDMap {
		if prov.Type == "lake" {
			for _, p := range prov.PixelCoords {
				img.Set(p.X, p.Y, waterColor)
			}
		}
	}

	// Draw state borders.
	stateBorderColor := color.RGBA{158, 158, 158, 255}
	for _, s := range statesMap {
		for _, p := range s.PixelCoords {
			_, exists := s.PixelCoordsMap[image.Point{p.X + 1, p.Y}]
			if !exists {
				img.Set(p.X+1, p.Y, stateBorderColor)
			}
			_, exists = s.PixelCoordsMap[image.Point{p.X, p.Y + 1}]
			if !exists {
				img.Set(p.X, p.Y+1, stateBorderColor)
			}
			_, exists = s.PixelCoordsMap[image.Point{p.X - 1, p.Y}]
			if !exists {
				img.Set(p.X, p.Y, stateBorderColor)
			}
			_, exists = s.PixelCoordsMap[image.Point{p.X, p.Y - 1}]
			if !exists {
				img.Set(p.X, p.Y, stateBorderColor)
			}
		}
	}

	// Init font.
	c, err := initFont(img)
	if err != nil {
		return err
	}

	//Draw state infrastructure values.
	for _, s := range statesMap {
		n := strconv.Itoa(s.Infrastructure)
		offset := 0
		if n != "" {
			offset = (len(n)*charWidth - strings.Count(n, "1") + len(n) - 1) / 2
		}
		err := addLabel(img, c, s.CenterPoint.X-offset, s.CenterPoint.Y+charHeight/2+1, 10.0, n)
		if err != nil {
			return err
		}
	}

	// Save image as PNG.
	out, err := os.Create("./infrastructure_map.png")
	if err != nil {
		return err
	}
	err = png.Encode(out, img)
	if err != nil {
		return err
	}
	fmt.Printf("%s: Saved 'infrastructure_map.png'\n", time.Since(startTime))

	return nil
}

func generateSmallProvincesMap(threshold int) error {
	fmt.Printf("%s: Generating small provinces map...\n", time.Since(startTime))

	// Create empty image and fill it with blue color (water).
	img := image.NewRGBA(provincesImageSize)
	draw.Draw(img, img.Bounds(), &image.Uniform{waterColor}, image.ZP, draw.Src)

	// Save small provinces in a list.
	smallProvinceList := []int{}

	// Draw land province shapes.
	for _, prov := range provincesIDMap {
		if prov.Type == "land" && prov.ID > 0 {
			fillCol := color.RGBA{255, 255, 255, 255}
			if len(prov.PixelCoords) < threshold {
				smallProvinceList = append(smallProvinceList, prov.ID)
				fillCol = generateRandomLightColor()
			}
			for _, p := range prov.PixelCoords {
				img.Set(p.X, p.Y, fillCol)
			}
		}
	}

	// Sort the province list.
	sort.Ints(smallProvinceList)

	// Create text file.
	f, err := os.OpenFile("small_provinces_list.txt", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0775)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write small provinces into files.
	s := "ID\tSTATE\tCOORDS\tPX_SIZE\n"
	if _, err = f.WriteString(s); err != nil {
		return err
	}
	for _, pID := range smallProvinceList {
		prov := provincesIDMap[pID]
		s := strconv.Itoa(prov.ID) + "\t" + strconv.Itoa(prov.State.ID) + "\t(" + strconv.Itoa(prov.CenterPoint.X) + "," + strconv.Itoa(prov.CenterPoint.Y) + ")\t" + strconv.Itoa(len(prov.PixelCoords)) + "\n"
		if _, err = f.WriteString(s); err != nil {
			return err
		}
	}
	fmt.Printf("%s: Saved 'small_provinces_list.txt'\n", time.Since(startTime))

	// Save image as PNG.
	out, err := os.Create("./small_provinces_map.png")
	if err != nil {
		return err
	}
	err = png.Encode(out, img)
	if err != nil {
		return err
	}
	fmt.Printf("%s: Saved 'small_provinces_map.png'\n", time.Since(startTime))

	// Scale image up.
	dst := image.NewRGBA(image.Rect(0, 0, img.Bounds().Max.X*4, img.Bounds().Max.Y*4))
	draw.NearestNeighbor.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	img = dst

	// Draw province borders.
	provinceBorderColor := color.RGBA{158, 158, 158, 255}
	for _, prov := range provincesIDMap {
		for _, p := range prov.PixelCoords {
			_, exists := prov.PixelCoordsMap[image.Point{p.X + 1, p.Y}]
			if !exists {
				img.Set(p.X*4+3, p.Y*4, provinceBorderColor)
				img.Set(p.X*4+3, p.Y*4+1, provinceBorderColor)
				img.Set(p.X*4+3, p.Y*4+2, provinceBorderColor)
				img.Set(p.X*4+3, p.Y*4+3, provinceBorderColor)
			}
			_, exists = prov.PixelCoordsMap[image.Point{p.X, p.Y + 1}]
			if !exists {
				img.Set(p.X*4, p.Y*4+3, provinceBorderColor)
				img.Set(p.X*4+1, p.Y*4+3, provinceBorderColor)
				img.Set(p.X*4+2, p.Y*4+3, provinceBorderColor)
				img.Set(p.X*4+3, p.Y*4+3, provinceBorderColor)
			}
		}
	}

	// Draw state borders.
	stateBorderColor := color.RGBA{255, 0, 0, 255}
	for _, s := range statesMap {
		for _, p := range s.PixelCoords {
			_, exists := s.PixelCoordsMap[image.Point{p.X + 1, p.Y}]
			if !exists {
				img.Set(p.X*4+3, p.Y*4, stateBorderColor)
				img.Set(p.X*4+3, p.Y*4+1, stateBorderColor)
				img.Set(p.X*4+3, p.Y*4+2, stateBorderColor)
				img.Set(p.X*4+3, p.Y*4+3, stateBorderColor)
			}
			_, exists = s.PixelCoordsMap[image.Point{p.X, p.Y + 1}]
			if !exists {
				img.Set(p.X*4, p.Y*4+3, stateBorderColor)
				img.Set(p.X*4+1, p.Y*4+3, stateBorderColor)
				img.Set(p.X*4+2, p.Y*4+3, stateBorderColor)
				img.Set(p.X*4+3, p.Y*4+3, stateBorderColor)
			}
			_, exists = s.PixelCoordsMap[image.Point{p.X - 1, p.Y}]
			if !exists {
				img.Set(p.X*4-1, p.Y*4, stateBorderColor)
				img.Set(p.X*4-1, p.Y*4+1, stateBorderColor)
				img.Set(p.X*4-1, p.Y*4+2, stateBorderColor)
				img.Set(p.X*4-1, p.Y*4+3, stateBorderColor)
			}
			_, exists = s.PixelCoordsMap[image.Point{p.X, p.Y - 1}]
			if !exists {
				img.Set(p.X*4, p.Y*4-1, stateBorderColor)
				img.Set(p.X*4+1, p.Y*4-1, stateBorderColor)
				img.Set(p.X*4+2, p.Y*4-1, stateBorderColor)
				img.Set(p.X*4+3, p.Y*4-1, stateBorderColor)
			}
		}
	}

	// Init font.
	c, err := initFont(img)
	if err != nil {
		return err
	}

	//Draw province IDs.
	for _, p := range provincesIDMap {
		n := strconv.FormatInt(int64(p.ID), 10)
		offset := 0
		if n != "" {
			offset = (len(n)*charWidth - strings.Count(n, "1") + len(n) - 1) / 2
		}
		err := addLabel(img, c, p.CenterPoint.X*4-offset, p.CenterPoint.Y*4+charHeight/2+1, 10.0, n)
		if err != nil {
			return err
		}
	}

	// Save image as PNG.
	out, err = os.Create("./small_provinces_map_x4.png")
	if err != nil {
		return err
	}
	err = png.Encode(out, img)
	if err != nil {
		return err
	}
	fmt.Printf("%s: Saved 'small_provinces_map_x4.png'\n", time.Since(startTime))

	return nil
}

func generateColorShuffledProvinceMap() error {
	fmt.Printf("%s: Generating color shuffled province map...\n", time.Since(startTime))

	// Create empty image and fill it with blue color (water).
	img := image.NewRGBA(provincesImageSize)
	draw.Draw(img, img.Bounds(), &image.Uniform{waterColor}, image.ZP, draw.Src)

	newProvincesRGBMap := make(map[color.Color]*Province)

	// Draw land province shapes.
	for _, prov := range provincesIDMap {
		if prov.ID == 0 {
			newProvincesRGBMap[color.RGBA{0, 0, 0, 0}] = prov
			continue
		}

		fillCol := generateRandomProvinceColor(prov)
		isColorUnique := false
		for !isColorUnique {
			_, ok := newProvincesRGBMap[fillCol]
			if ok {
				fillCol = generateRandomProvinceColor(prov)
			} else {
				closeAdjacentColor := true
				i := 0
				for closeAdjacentColor && (i < 256) {
					closeAdjacentColor = false
					for _, a := range prov.AdjacentTo {
						if (a.RenderColor != color.RGBA{0, 0, 0, 0}) && (isColorClose(fillCol, a.RenderColor)) {
							closeAdjacentColor = true
							i++
							continue
						}
					}
				}

				prov.RenderColor = fillCol
				newProvincesRGBMap[fillCol] = prov
				isColorUnique = true
			}
		}
		for _, p := range prov.PixelCoords {
			img.Set(p.X, p.Y, fillCol)
		}
	}

	// Save image as PNG.
	out, err := os.Create("./color_shuffled_province_map.png")
	if err != nil {
		return err
	}
	err = png.Encode(out, img)
	if err != nil {
		return err
	}
	fmt.Printf("%s: Saved 'color_shuffled_province_map.png'\n", time.Since(startTime))

	// Write new definition.csv.
	var IDs []int
	for id := range provincesIDMap {
		IDs = append(IDs, id)
	}
	sort.Ints(IDs)

	f, err := os.OpenFile("definition.csv", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0775)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, id := range IDs {
		p := provincesIDMap[id]
		s := fmt.Sprintf("%v;%v;%v;%v;%v;%v;%v;%v\r\n", p.ID, p.RenderColor.R, p.RenderColor.G, p.RenderColor.B, p.Type, p.IsCoastal, p.Terrain, p.Continent)
		if _, err = f.WriteString(s); err != nil {
			return err
		}
	}
	fmt.Printf("%s: Saved 'definition.csv'\n", time.Since(startTime))

	return nil
}

func generateImpassableMap() error {
	fmt.Printf("%s: Generating impassable terrain map...\n", time.Since(startTime))

	// Create empty image.
	img := image.NewRGBA(provincesImageSize)
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{0, 0, 0, 0}}, image.ZP, draw.Src)

	// Draw state shapes.
	for _, s := range statesMap {
		if s.IsImpassable {
			for _, p := range s.PixelCoords {
				img.Set(p.X, p.Y, color.RGBA{255, 255, 255, 255})
			}
		}
	}

	// Save image as PNG.
	out, err := os.Create("./impassable_map.png")
	if err != nil {
		return err
	}
	err = png.Encode(out, img)
	if err != nil {
		return err
	}
	fmt.Printf("%s: Saved 'impassable_map.png'\n", time.Since(startTime))
	return nil
}

func generateRandomProvinceColor(prov *Province) color.RGBA {
	fillCol := generateRandomLandColor()
	if prov.Type != "land" {
		fillCol = generateRandomSeaColor()
	}
	return fillCol
}

func generateRandomLandColor() color.RGBA {
	maxR := 255
	minR := 33
	maxG := 255
	minG := 33
	maxB := 255
	minB := 0
	return color.RGBA{uint8(rand.Intn(maxR-minR) + minR), uint8(rand.Intn(maxG-minG) + minG), uint8(rand.Intn(maxB-minB) + minB), 255}
}

func generateRandomSeaColor() color.RGBA {
	maxR := 16
	minR := 0
	maxG := 16
	minG := 0
	maxB := 192
	minB := 16
	return color.RGBA{uint8(rand.Intn(maxR-minR) + minR), uint8(rand.Intn(maxG-minG) + minG), uint8(rand.Intn(maxB-minB) + minB), 255}
}

func generateProvinceContinentValues(continentsPath string) error {
	fmt.Printf("%s: Generating new province continent values...\n", time.Since(startTime))

	c1 := color.RGBA{75, 43, 7, 255}     // brown: west_coast
	c2 := color.RGBA{255, 255, 255, 255} // white: northern_reaches
	c3 := color.RGBA{11, 103, 0, 255}    // green: land_of_titans
	c4 := color.RGBA{222, 221, 39, 255}  // yellow: midwest
	c5 := color.RGBA{169, 45, 45, 255}   // red: east_coast
	c6 := color.RGBA{0, 210, 235, 255}   // cyan: caribbean_expanse

	continentsFile, err := os.Open(filepath.FromSlash(continentsPath))
	if err != nil {
		return err
	}
	defer continentsFile.Close()
	continentsImage, _, err := image.Decode(continentsFile)
	if err != nil {
		return err
	}

	// Write new definition.csv.
	var IDs []int
	for id := range provincesIDMap {
		IDs = append(IDs, id)
	}
	sort.Ints(IDs)

	f, err := os.OpenFile("definition.csv", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0775)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, id := range IDs {
		p := provincesIDMap[id]
		continent := 0

		if p.Terrain != "ocean" {
			var col1 color.RGBA
			for _, px := range p.PixelCoords {
				col2 := continentsImage.At(px.X, px.Y)
				r1, g1, b1, a1 := col1.RGBA()
				r2, g2, b2, a2 := col2.RGBA()

				if a1 != 0 && r1 != r2 && g1 != g2 && b1 != b2 && a1 != a2 {
					return fmt.Errorf("Different continent colors in province %v at %v", p.ID, px)
				}

				col1 = col2.(color.RGBA)

				switch fmt.Sprintf("%v", col2) {
				case fmt.Sprintf("%v", c1):
					continent = 1
				case fmt.Sprintf("%v", c2):
					continent = 2
				case fmt.Sprintf("%v", c3):
					continent = 3
				case fmt.Sprintf("%v", c4):
					continent = 4
				case fmt.Sprintf("%v", c5):
					continent = 5
				case fmt.Sprintf("%v", c6):
					continent = 6
				}
			}
		}

		s := fmt.Sprintf("%v;%v;%v;%v;%v;%v;%v;%v\r\n", p.ID, p.RGB.R, p.RGB.G, p.RGB.B, p.Type, p.IsCoastal, p.Terrain, continent)
		if _, err = f.WriteString(s); err != nil {
			return err
		}
	}
	fmt.Printf("%s: Saved 'definition.csv'\n", time.Since(startTime))

	return nil
}

func containsPoint(s []image.Point, a image.Point) bool {
	for _, b := range s {
		if b.X == a.X && b.Y == a.Y {
			return true
		}
	}
	return false
}

var csvFileContents []string

func createStateCenterPointsCSV() error {
	fmt.Printf("%s: Generating CSV output for state centers...\n", time.Since(startTime))
	local_path := localModFilesPath + "/common/on_actions"
	_, err := os.Stat(local_path)
	if os.IsNotExist(err) {
		errDir := os.MkdirAll(local_path, 0755)
		if errDir != nil {
			log.Fatal(err)
		}

	}
	f, err := os.Create(local_path + "/state_centers_on_actions.txt")
	defer f.Close()
	if err != nil {
		log.Fatalln("failed to open file", err)
	}
	csvFileContents = append(csvFileContents, `
	on_actions = {
		on_startup = {
			effect = {
	`)
	for _, s := range statesMap {
		id := fmt.Sprintf("%v", s.ID)
		centerX := fmt.Sprintf("%v", s.CenterPointRec.X)
		centerY := fmt.Sprintf("%v", s.CenterPointRec.Y)
		//rowSlice := []string{id, centerX, centerY}
		rowSlice := fmt.Sprintf(`
			%[1]v = {
				set_variable = { map_x_position = %[2]v }
				set_variable = { map_y_position = %[3]v }
			}
		`, id, centerX, centerY)
		csvFileContents = append(csvFileContents, rowSlice)
	}
	csvFileContents = append(csvFileContents, `
			}
		}
	}
	`)
	for _, w := range csvFileContents {
		_, err := f.WriteString(w)

		if err != nil {
			log.Fatal(err)
		}
	}
	return nil
}

var fileContents []string

var fileHeader string = `
### Autogenerated file. Please regenerate through hoi4geoparser.

spriteTypes = {
	`
var spriteHeader = `spriteType = {
	name = "GFX_custom_map_state_image_`
var spriteTexture = `
	textureFile = "gfx/interface/gui/custom_map_mode/state_images/state_image_`
var spriteEffect = `
	effectFile = "gfx/FX/stateshape.lua"
`
var spriteTextureHashed = `
	textureFile = "gfx/interface/gui/custom_map_mode/state_images/state_image_hashed_`
var spriteFrames = `
	noOfFrames = `
var spriteFooter = `
	transparencecheck = yes
}

`

func createStatePngFiles() error {
	fmt.Printf("%s: Creating state images! \n", time.Since(startTime))
	local_path := gfxFilePath + "/state_images"
	// Set up folder
	_, err := os.Stat(local_path)
	if os.IsNotExist(err) {
		errDir := os.MkdirAll(local_path, 0755)
		if errDir != nil {
			log.Fatal(err)
		}

	}
	fileContents = append(fileContents, fileHeader)
	// Loop!
	fmt.Printf("%s: Generating State Images for %v states using %v colors\n", time.Since(startTime), len(statesMap), len(colors))
	// We want to tile the mask to the size of our image
	maskImgSlice, err := png.Decode(bytes.NewReader(alphaMaskImage))
	if err != nil {
		processError(err)
	}
	maskImgSize := image.Rect(0, 0, provincesImageSize.Dx(), provincesImageSize.Dy())
	maskImg := image.NewRGBA(maskImgSize)
	tile_x := provincesImageSize.Max.X / maskImgSlice.Bounds().Max.X
	tile_y := provincesImageSize.Max.Y / maskImgSlice.Bounds().Max.Y
	for tile := 0; tile < tile_x; tile++ {
		offsetX := (tile * maskImgSlice.Bounds().Max.X) + 1
		for tileH := 0; tileH < tile_y; tileH++ {
			offsetY := (tileH * maskImgSlice.Bounds().Max.Y) + 1
			draw.Draw(maskImg, image.Rect(offsetX, offsetY, offsetX+maskImgSlice.Bounds().Dx(), offsetY+maskImgSlice.Bounds().Dy()), maskImgSlice, image.Pt(0, 0), draw.Src)
		}
	}
	fmt.Printf("Iterating.....\n")
	for _, s := range statesMap {
		// Find largest dimension
		var xMin int = s.PixelCoords[0].X
		var xMax int
		var yMin int = s.PixelCoords[0].Y
		var yMax int
		for _, c := range s.PixelCoords {
			if c.X < xMin {
				xMin = c.X
			}
			if c.X > xMax {
				xMax = c.X
			}
			if c.Y < yMin {
				yMin = c.Y
			}
			if c.Y > yMax {
				yMax = c.Y
			}
		}
		xRange := xMax - xMin

		// add 2x more space onto the end to create a strip
		imgSize := image.Rect(xMin, yMin, (xMax + (image_repeats-1)*(xRange+1) + 1), yMax+1)
		widthX := (xRange + 1) / 2
		heightY := (yMax - yMin + 1) / 2
		s.CenterPointRec = image.Point{widthX + xMin, heightY + yMin}
		img := image.NewRGBA(imgSize)
		alphaCol := color.RGBA{0, 0, 0, 0}
		draw.Draw(img, img.Bounds(), &image.Uniform{alphaCol}, image.Pt(0, 0), draw.Src)
		// We want the hash to start the same as the image
		hashSize := image.Rect(xMin, yMin, (xMax + (image_repeats_hash-1)*(xRange+1)), yMax)
		hash_img := image.NewRGBA(hashSize)
		draw.Draw(hash_img, hash_img.Bounds(), &image.Uniform{alphaCol}, image.Pt(0, 0), draw.Src)
		// Iterate through main images
		for tileNumber := 0; tileNumber < image_repeats; tileNumber++ {
			offset_X := (xRange + 1) * tileNumber
			for _, p := range s.PixelCoords {
				img.Set(p.X+offset_X, p.Y, colors[tileNumber])
				// Set up borders
				_, exists := s.PixelCoordsMap[image.Point{p.X + 1, p.Y}]
				if !exists {
					img.Set(p.X+offset_X, p.Y, color.Black)
				}
				_, exists = s.PixelCoordsMap[image.Point{p.X - 1, p.Y}]
				if !exists {
					img.Set(p.X+offset_X, p.Y, color.Black)
				}
				_, exists = s.PixelCoordsMap[image.Point{p.X, p.Y + 1}]
				if !exists {
					img.Set(p.X+offset_X, p.Y, color.Black)
				}
				_, exists = s.PixelCoordsMap[image.Point{p.X, p.Y - 1}]
				if !exists {
					img.Set(p.X+offset_X, p.Y, color.Black)
				}
			}
		}
		/// Iterate through hash images
		for tileNumber := 0; tileNumber < image_repeats_hash; tileNumber++ {
			offset_X := (xRange + 1) * tileNumber
			//fmt.Printf("TileNumer: %v, Tiles %v OffsetX %v size %v \n", tileNumber, image_repeats, offset_X, img.Bounds().Size().X)
			//offset_Y := ()
			for _, p := range s.PixelCoords {
				hash_img.Set(p.X+offset_X, p.Y, hash_colors[tileNumber])
			}
		}
		idString := fmt.Sprintf("%v", s.ID)
		out, err := os.Create((local_path + "/state_image_" + idString + ".png"))
		if err != nil {
			processError(err)
		}
		defer out.Close()
		//Disgusting type casting because i don't know how to do it otherwise
		xScale_O := int(math.Round(float64(img.Bounds().Dx()) * main_scale))
		YScale_O := int(math.Round(float64(img.Bounds().Dy()) * main_scale))
		if main_scale != 1.0 {
			scaledOriginal := image.NewNRGBA(image.Rect(0, 0, xScale_O, YScale_O))
			draw.NearestNeighbor.Scale(scaledOriginal, scaledOriginal.Rect, img, img.Rect, draw.Over, nil)
			err = png.Encode(out, scaledOriginal)
		} else {
			err = png.Encode(out, img)
		}
		if err != nil {
			log.Fatal(err)
		}
		out.Close()
		//Encode a scaled-down hashed version of the slices
		xScale_H := int(math.Round(float64(hash_img.Bounds().Dx()) * hash_scale))
		YScale_H := int(math.Round(float64(hash_img.Bounds().Dy()) * hash_scale))
		scaledHash := image.NewNRGBA(image.Rect(0, 0, xScale_H, YScale_H))
		//Decode packed mask
		//maskImg := image.NewNRGBA(image.Rect(0, 0, 500, 500))
		//draw.Draw(maskImg, maskImg.Rect, &image.Uniform{color.Opaque}, image.Pt(0, 0), draw.Src)

		//Apply mask image
		dest_img := image.NewNRGBA(hash_img.Rect)
		draw.DrawMask(dest_img, dest_img.Rect, hash_img, hash_img.Rect.Min, maskImg, image.Pt(0, 0), draw.Over)
		//Testing mask
		//draw.Draw(dest_img, dest_img.Rect, maskImg, hash_img.Rect.Min, draw.Over)
		draw.NearestNeighbor.Scale(scaledHash, scaledHash.Rect, dest_img, dest_img.Rect, draw.Over, nil)
		scaleOut, err := os.Create((local_path + "/state_image_hashed_" + idString + ".png"))
		if err != nil {
			log.Fatal(err)
		}
		defer scaleOut.Close()
		png.Encode(scaleOut, scaledHash)
		scaleOut.Close()
		// create slice of file
		fileSection := spriteHeader + idString + "\"\n" + spriteTexture + idString + ".png\"" + spriteEffect + spriteFrames + fmt.Sprintf("%v", image_repeats) + spriteFooter
		fileSectionHash := spriteHeader + "hashed_" + idString + "\"\n" + spriteTextureHashed + idString + ".png\"" + spriteEffect + spriteFrames + fmt.Sprintf("%v", image_repeats_hash) + spriteFooter
		fileContents = append(fileContents, fileSection)
		fileContents = append(fileContents, fileSectionHash)
		fmt.Printf("State %v finished!\n", s.ID)
	}
	fileContents = append(fileContents, "\n}")
	local_path = localModFilesPath + "/interface"
	// Set up folder
	_, err = os.Stat(local_path)
	if os.IsNotExist(err) {
		errDir := os.MkdirAll(local_path, 0755)
		if errDir != nil {
			log.Fatal(err)
		}

	}
	f, err := os.Create(local_path + "/custom_states_generated_state_images.gfx")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	for _, w := range fileContents {
		_, err := f.WriteString(w)

		if err != nil {
			log.Fatal(err)
		}
	}
	f.Close()
	fmt.Print("GFX file finished\n")
	vanillaImage, _ := png.Decode(bytes.NewReader(vanilla_terrain_file))
	vanillaFile, err := os.Create(gfxFilePath + "/big_map_bg.png")
	if err != nil {
		processError(err)
	}
	png.Encode(vanillaFile, vanillaImage)
	fmt.Print("Vanilla Background Saved\n")
	return nil
}
func createDebugTerrainType() {
	var terrain TerrainType
	terrain.color = 3
	img, err := createTerrainImage(atlasTextureMap[1])
	terrain.texture_img = img
	terrain.texture_index = 0
	if err != nil {
		processError(err)
	}
}
func parseTerrainTypes() error {
	// Open terrain image
	terrainFile, err := os.Open(filepath.FromSlash(terrainPath))
	if err != nil {
		return err
	}
	defer terrainFile.Close()
	terrainImage, err := bmp.DecodeConfig(terrainFile)
	if err != nil {
		return err
	}
	terrainColors, ok := terrainImage.ColorModel.(color.Palette)
	if ok {
		fmt.Printf("BMP Color index 0: %v\n", terrainColors[0])
	} else {
		fmt.Print("Not a palette")
	}
	return nil
}

/// Input a point described in the atlasTextureMap, 0-15 > x,y
func createTerrainImage(textureIndex image.Point) (image.Image, error) {
	//Open Atlas
	reader, err := os.Open(atlasPath)
	if err != nil {
		fmt.Print("Atlas Texture Missing")
		processError(err)
	}
	defer reader.Close()
	atlasImage, _, err := image.Decode(reader)
	if err != nil {
		processError(err)
	}
	//Pull tile from atlas
	tileR := atlasTileSize.Add(textureIndex)
	tileImg, err := cropImage(atlasImage, tileR)
	// new image
	atlasOverlay := image.NewNRGBA(provincesImageSize)
	var atlasTileX int = int(math.Ceil(float64(provincesImageSize.Size().X) / float64(atlasTileSize.Size().X)))
	var atlasTileY int = int(math.Ceil(float64(provincesImageSize.Size().Y) / float64(atlasTileSize.Size().Y)))
	fmt.Printf("X size %v, Y size %v \n", atlasTileX, atlasTileY)
	//drawPoint := image.Pt(0, 0)
	r := tileImg.Bounds()
	for x := 0; x < atlasTileX; x++ {
		for y := 0; y < atlasTileY; y++ {
			fmt.Println(r)
			//drawPoint.Add(image.Pt(atlasTileSize.Size().X*x, atlasTileSize.Size().Y*y))
			fmt.Printf("Drawing rectangle at %v moved by %v \n", r, atlasTileSize.Size())
			draw.Draw(atlasOverlay, r, tileImg, image.Pt(0, 0), draw.Src)
			// add across by atlas tile y
			r = r.Add(image.Point{0, atlasTileSize.Size().Y})
		}
		// make a new rect starting at the old max, then add the tile size for new max, 0 out Y location
		r = image.Rect(r.Max.X, 0, atlasTileSize.Max.X+r.Max.X, atlasTileSize.Max.Y)
	}
	// Write atlas output (DEBUG)
	writer, err := os.Create("atlas_output.png")
	if err != nil {
		processError(err)
	}
	png.Encode(writer, atlasOverlay)
	return atlasOverlay, nil
}
func cropImage(img image.Image, crop image.Rectangle) (image.Image, error) {
	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}

	// img is an Image interface. This checks if the underlying value has a
	// method called SubImage. If it does, then we can use SubImage to crop the
	// image.
	simg, ok := img.(subImager)
	if !ok {
		return nil, fmt.Errorf("image does not support cropping")
	}

	return simg.SubImage(crop), nil
}
