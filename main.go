package main

import (
	"bufio"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
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
	"github.com/hotei/bmp"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
)

var modPath = "c:/Users/admin/Documents/Paradox Interactive/Hearts of Iron IV/mod/oldworldblues_mexico"

// var modPath = "d:/Games/SteamApps/common/Hearts of Iron IV"
var definitionsPath = modPath + "/map/definition.csv"
var adjacenciesPath = modPath + "/map/adjacencies.csv"
var provincesPath = modPath + "/map/provinces.bmp"
var terrainPath = modPath + "/map/terrain.bmp"
var heightmapPath = modPath + "/map/heightmap.bmp"
var statesPath = modPath + "/history/states"
var provincesIDMap = make(map[int]*Province)
var provincesRGBMap = make(map[color.Color]*Province)
var statesMap = make(map[int]*State)
var rStateID = regexp.MustCompile(`(?s:.*id.*?=.*?(\d+).*)`)
var rStateName = regexp.MustCompile(`(?s:.*name.*?=.*?\"(.+?)\".*)`)
var rStateManpower = regexp.MustCompile(`(?s:.*manpower.*?=.*?(\d+).*)`)
var rStateProvinces = regexp.MustCompile(`(?s:.*provinces.*?=.*?{.*?([0-9 ]+).*?}.*)`)
var rSpace = regexp.MustCompile(`\s+`)
var mapScalePixelToKm = 7.114
var provincesImageSize image.Rectangle
var waterColor = color.RGBA{68, 107, 163, 255}

func main() {
	// Track start time for benchmarking.
	startTime := time.Now()

	// Parse  definition.csv for provinces.
	err := parseDefinitions()
	if err != nil {
		panic(err)
	}

	// Parse  adjacencies.csv for province connections and impassable borders.
	err = parseAdjacencies()
	if err != nil {
		panic(err)
	}

	// Parse provinces.bmp for province adjacency.
	err = parseProvinces()
	if err != nil {
		panic(err)
	}

	// Find the center points of each province.
	findProvincesCenterPoints()

	// Parse state files.
	err = parseStateFiles()
	if err != nil {
		panic(err)
	}

	// Parse states provinces.
	parseStatesProvinces()

	// Parse states distance to other states.
	parseStatesDistanceToOtherStates()

	// // Write the output file.
	// err = saveGeoData()
	// if err != nil {
	// 	panic(err)
	// }

	// // Generate state ID map.
	// err = generateSateIDMap()
	// if err != nil {
	// 	panic(err)
	// }

	// // Generate province map.
	// err = generateProvinceMap()
	// if err != nil {
	// 	panic(err)
	// }

	// // Generate province ID map.
	// err = generateProvinceIDMap()
	// if err != nil {
	// 	panic(err)
	// }

	// // Generate manpower map.
	// err = generateManpowerMap()
	// if err != nil {
	// 	panic(err)
	// }

	// // Generate sea province map.
	// err = generateSeaProvinceMap()
	// if err != nil {
	// 	panic(err)
	// }

	// // Generate province-based terrain map.
	// err = generateProvinceBasedTerrainMap()
	// if err != nil {
	// 	panic(err)
	// }

	// Generate province-based heightmap threshold map.
	err = generateProvinceBasedHeightmapThresholdMap()
	if err != nil {
		panic(err)
	}

	// Print out elapsed time.
	elapsedTime := time.Since(startTime)
	fmt.Printf("Elapsed time: %s\n", elapsedTime)
}

// Province represents an in-game province with all parsed data in it.
type Province struct {
	ID           int
	RGB          color.RGBA
	Type         string // "land" or "sea"
	IsCoastal    bool
	Terrain      string
	Continent    int
	PixelCoords  map[image.Point]image.Point
	CenterPoint  image.Point
	AdjacentTo   map[int]*Province
	ConnectedTo  map[int]*Province
	ImpassableTo map[int]*Province
}

// State represents an in-game state with all parsed data in it.
type State struct {
	ID           int
	Name         string
	Manpower     int
	IsCoastal    bool
	Continent    int
	PixelCoords  map[image.Point]image.Point
	CenterPoint  image.Point
	Provinces    map[int]*Province
	DistanceTo   map[int]int // Distance to other states.
	AdjacentTo   map[int]*State
	ConnectedTo  map[int]*State
	ImpassableTo map[int]*State
	RenderColor  color.RGBA
}

// ReadLines reads a whole file
// and returns a slice of its lines.
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func parseDefinitions() error {
	fmt.Println("Parsing definition.csv...")
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
		return p, errors.New("\"" + definitionsPath + "\": " + s + ": must contain 8 fields")
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
	p.PixelCoords = make(map[image.Point]image.Point)
	p.AdjacentTo = make(map[int]*Province)
	p.ConnectedTo = make(map[int]*Province)
	p.ImpassableTo = make(map[int]*Province)

	return p, nil
}

func parseAdjacencies() error {
	fmt.Println("Parsing adjacencies.csv...")
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
	fmt.Println("Parsing provinces.bmp...")
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

			// Add pixel coordinates to the province that has this RGB value.
			provincesRGBMap[c].PixelCoords[image.Point{x, y}] = image.Point{x, y}

			// Find out the color of the adjacent right and bottom pixels.
			if x < provincesImage.Bounds().Max.X-1 {
				e = provincesImage.At(x+1, y)
			}
			if y < provincesImage.Bounds().Max.Y-1 {
				s = provincesImage.At(x, y+1)
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
	fmt.Println("Calculating provinces center point coordinates...")
	for _, p := range provincesIDMap {
		p.CenterPoint = findCenterPoint(p.PixelCoords)
	}
}

func findCenterPoint(coords map[image.Point]image.Point) image.Point {
	x := 0
	y := 0

	for _, c := range coords {
		x += c.X
		y += c.Y
	}
	return image.Point{int(math.Round(float64(x) / float64(len(coords)))), int(math.Round(float64(y) / float64(len(coords))))}
}

func parseStateFiles() error {
	fmt.Println("Parsing state files...")
	stateFiles, err := filepath.Glob(filepath.FromSlash(statesPath) + string(os.PathSeparator) + "*.txt")
	if err != nil {
		return err
	}
	for _, s := range stateFiles {
		state, err := parseState(s)
		if err != nil {
			return err
		}
		statesMap[state.ID] = &state
	}
	return nil
}

func parseState(path string) (state State, err error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return state, err
	}
	s := strings.Replace(string(b), "\r\n", "\n", -1)
	state.ID, err = strconv.Atoi(rStateID.FindStringSubmatch(s)[1])
	if err != nil {
		return state, err
	}
	state.Name = rStateName.FindStringSubmatch(s)[1]
	state.Manpower, err = strconv.Atoi(rStateManpower.FindStringSubmatch(s)[1])
	if err != nil {
		return state, err
	}
	state.Provinces = make(map[int]*Province)
	provinces := strings.Split(strings.TrimSpace(rSpace.ReplaceAllString(rStateProvinces.FindStringSubmatch(s)[1], " ")), " ")
	for _, p := range provinces {
		pID, err := strconv.Atoi(p)
		if err != nil {
			return state, err
		}
		state.Provinces[pID] = provincesIDMap[pID]
	}
	state.Continent = -1
	state.PixelCoords = make(map[image.Point]image.Point)
	state.DistanceTo = make(map[int]int)
	state.AdjacentTo = make(map[int]*State)
	state.ConnectedTo = make(map[int]*State)
	state.ImpassableTo = make(map[int]*State)

	return state, nil
}

func parseStatesProvinces() {
	fmt.Println("Parsing provinces in each state...")
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
				s1.PixelCoords[pc] = pc
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
		}
	}

	for _, s1 := range statesMap {
		// Find the center point of the state.
		s1.CenterPoint = findCenterPoint(s1.PixelCoords)

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
	fmt.Println("Calculating distance between each state...")
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

func saveGeoData() error {
	fmt.Println("Writing the output file...")
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
		// Write the state id into the output file.
		_, err = f.WriteString("\t\t\t" + strconv.Itoa(sID) + " = {\n")
		if err != nil {
			return err
		}

		if len(statesMap[sID].ConnectedTo) > 0 {
			// Sort the DistanceTo map.
			statesConnectedToIDs := sortedKeySliceFromStateMap(statesMap[sID].ConnectedTo)
			// Iterate over all states from ConnectedTo map in ID sorted order.
			for _, cID := range statesConnectedToIDs {
				// Write the connected_to@STATE variables.
				_, err = f.WriteString("\t\t\t\tset_variable = { connected_to@" + strconv.Itoa(cID) + " = 1 }\n")
				if err != nil {
					return err
				}
			}
		}

		if len(statesMap[sID].ImpassableTo) > 0 {
			// Sort the DistanceTo map.
			statesImpassableToIDs := sortedKeySliceFromStateMap(statesMap[sID].ImpassableTo)
			// Iterate over all states from ImpassableTo map in ID sorted order.
			for _, aID := range statesImpassableToIDs {
				// Write the impassable_to@STATE variables.
				_, err = f.WriteString("\t\t\t\tset_variable = { impassable_to@" + strconv.Itoa(aID) + " = 1 }\n")
				if err != nil {
					return err
				}
			}
		}

		// Sort the DistanceTo map.
		statesDistanceToIDs := sortedKeySliceFromIntMap(statesMap[sID].DistanceTo)
		// Iterate over all states from DistanceTO map in ID sorted order.
		for _, dID := range statesDistanceToIDs {
			// Write the distance_to@STATE variables.
			_, err = f.WriteString("\t\t\t\tset_variable = { distance_to@" + strconv.Itoa(dID) + " = " + strconv.Itoa(statesMap[sID].DistanceTo[dID]) + " }\n")
			if err != nil {
				return err
			}
		}

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
	// fmt.Printf("%v %v  |  %e  %f  %v\n", a, b, d, d, d < 1000000000000000000000000000000000000)
	return d < 1000000000000000000000000000000000000
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
	pt := freetype.Pt(x, y+int(c.PointToFixed(size)>>6))
	if _, err := c.DrawString(label, pt); err != nil {
		return err
	}
	return nil
}

func generateSateIDMap() error {
	fmt.Println("Generating state ID map...")

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

	// Draw state borders.
	stateBorderColor := color.RGBA{128, 128, 128, 255}
	for _, s := range statesMap {
		for _, p := range s.PixelCoords {
			_, exists := s.PixelCoords[image.Point{p.X + 1, p.Y}]
			if !exists {
				img.Set(p.X+1, p.Y, stateBorderColor)
			}
			_, exists = s.PixelCoords[image.Point{p.X, p.Y + 1}]
			if !exists {
				img.Set(p.X, p.Y+1, stateBorderColor)
			}
			_, exists = s.PixelCoords[image.Point{p.X - 1, p.Y}]
			if !exists {
				img.Set(p.X, p.Y, stateBorderColor)
			}
			_, exists = s.PixelCoords[image.Point{p.X, p.Y - 1}]
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

	//Draw state IDs.
	for _, s := range statesMap {
		err := addLabel(img, c, s.CenterPoint.X-7, s.CenterPoint.Y-7, 10.0, strconv.FormatInt(int64(s.ID), 10))
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
	fmt.Println("Saved 'state_map_with_ids.png'")
	return nil
}

func initFont(img *image.RGBA) (*freetype.Context, error) {
	// Read the font data.
	fontBytes, err := ioutil.ReadFile("smallest_pixel-7.ttf")
	if err != nil {
		return nil, err
	}
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
	fmt.Println("Generating province map...")

	// Create empty image and fill it with blue color (water).
	img := image.NewRGBA(provincesImageSize)
	draw.Draw(img, img.Bounds(), &image.Uniform{waterColor}, image.ZP, draw.Src)

	// Draw state shapes.
	fillCol := color.RGBA{255, 255, 255, 255}
	for _, s := range statesMap {
		for _, p := range s.PixelCoords {
			img.Set(p.X, p.Y, fillCol)
		}
	}

	// Draw province borders.
	provinceBorderColor := color.RGBA{128, 128, 128, 255}
	for _, prov := range provincesIDMap {
		for _, p := range prov.PixelCoords {
			_, exists := prov.PixelCoords[image.Point{p.X + 1, p.Y}]
			if !exists {
				img.Set(p.X+1, p.Y, provinceBorderColor)
			}
			_, exists = prov.PixelCoords[image.Point{p.X, p.Y + 1}]
			if !exists {
				img.Set(p.X, p.Y+1, provinceBorderColor)
			}
		}
	}

	// Draw state borders.
	stateBorderColor := color.RGBA{255, 0, 0, 255}
	for _, s := range statesMap {
		for _, p := range s.PixelCoords {
			_, exists := s.PixelCoords[image.Point{p.X + 1, p.Y}]
			if !exists {
				img.Set(p.X+1, p.Y, stateBorderColor)
			}
			_, exists = s.PixelCoords[image.Point{p.X, p.Y + 1}]
			if !exists {
				img.Set(p.X, p.Y+1, stateBorderColor)
			}
			_, exists = s.PixelCoords[image.Point{p.X - 1, p.Y}]
			if !exists {
				img.Set(p.X, p.Y, stateBorderColor)
			}
			_, exists = s.PixelCoords[image.Point{p.X, p.Y - 1}]
			if !exists {
				img.Set(p.X, p.Y, stateBorderColor)
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
	fmt.Println("Saved 'province_map.png'")

	return nil
}

func generateProvinceIDMap() error {
	fmt.Println("Generating province ID map...")

	// Create empty image and fill it with blue color (water).
	img := image.NewRGBA(provincesImageSize)
	draw.Draw(img, img.Bounds(), &image.Uniform{waterColor}, image.ZP, draw.Src)

	// Draw state shapes.
	fillCol := color.RGBA{255, 255, 255, 255}
	for _, s := range statesMap {
		for _, p := range s.PixelCoords {
			img.Set(p.X, p.Y, fillCol)
		}
	}

	// Scale image up.
	dst := image.NewRGBA(image.Rect(0, 0, img.Bounds().Max.X*4, img.Bounds().Max.Y*4))
	draw.NearestNeighbor.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	img = dst

	// Draw province borders.
	provinceBorderColor := color.RGBA{128, 128, 128, 255}
	for _, prov := range provincesIDMap {
		for _, p := range prov.PixelCoords {
			_, exists := prov.PixelCoords[image.Point{p.X + 1, p.Y}]
			if !exists {
				img.Set(p.X*4+3, p.Y*4, provinceBorderColor)
				img.Set(p.X*4+3, p.Y*4+1, provinceBorderColor)
				img.Set(p.X*4+3, p.Y*4+2, provinceBorderColor)
				img.Set(p.X*4+3, p.Y*4+3, provinceBorderColor)
			}
			_, exists = prov.PixelCoords[image.Point{p.X, p.Y + 1}]
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
			_, exists := s.PixelCoords[image.Point{p.X + 1, p.Y}]
			if !exists {
				img.Set(p.X*4+3, p.Y*4, stateBorderColor)
				img.Set(p.X*4+3, p.Y*4+1, stateBorderColor)
				img.Set(p.X*4+3, p.Y*4+2, stateBorderColor)
				img.Set(p.X*4+3, p.Y*4+3, stateBorderColor)
			}
			_, exists = s.PixelCoords[image.Point{p.X, p.Y + 1}]
			if !exists {
				img.Set(p.X*4, p.Y*4+3, stateBorderColor)
				img.Set(p.X*4+1, p.Y*4+3, stateBorderColor)
				img.Set(p.X*4+2, p.Y*4+3, stateBorderColor)
				img.Set(p.X*4+3, p.Y*4+3, stateBorderColor)
			}
			_, exists = s.PixelCoords[image.Point{p.X - 1, p.Y}]
			if !exists {
				img.Set(p.X*4-1, p.Y*4, stateBorderColor)
				img.Set(p.X*4-1, p.Y*4+1, stateBorderColor)
				img.Set(p.X*4-1, p.Y*4+2, stateBorderColor)
				img.Set(p.X*4-1, p.Y*4+3, stateBorderColor)
			}
			_, exists = s.PixelCoords[image.Point{p.X, p.Y - 1}]
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
		err := addLabel(img, c, p.CenterPoint.X*4-7, p.CenterPoint.Y*4-7, 10.0, strconv.FormatInt(int64(p.ID), 10))
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
	fmt.Println("Saved 'province_id_map.png'")

	return nil
}

func generateManpowerMap() error {
	fmt.Println("Generating manpower map...")

	// Create empty image and fill it with blue color (water).
	img := image.NewRGBA(provincesImageSize)
	draw.Draw(img, img.Bounds(), &image.Uniform{waterColor}, image.ZP, draw.Src)

	// Find highest manpower value in a state.
	mpMin := 200000
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

	// Draw state borders.
	stateBorderColor := color.RGBA{128, 128, 128, 255}
	for _, s := range statesMap {
		for _, p := range s.PixelCoords {
			_, exists := s.PixelCoords[image.Point{p.X + 1, p.Y}]
			if !exists {
				img.Set(p.X+1, p.Y, stateBorderColor)
			}
			_, exists = s.PixelCoords[image.Point{p.X, p.Y + 1}]
			if !exists {
				img.Set(p.X, p.Y+1, stateBorderColor)
			}
			_, exists = s.PixelCoords[image.Point{p.X - 1, p.Y}]
			if !exists {
				img.Set(p.X, p.Y, stateBorderColor)
			}
			_, exists = s.PixelCoords[image.Point{p.X, p.Y - 1}]
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
		err := addLabel(img, c, s.CenterPoint.X-7, s.CenterPoint.Y-7, 10.0, intToString(s.Manpower))
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
	fmt.Println("Saved 'manpower_map.png'")

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
	fmt.Println("Generating sea province map...")

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
	fmt.Println("Saved 'sea_province_map.png'")

	return nil
}

func generateProvinceBasedTerrainMap() error {
	fmt.Println("Generating province-based terrain map...")

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
	fmt.Println("Saved 'province_based_terrain.png'")

	return nil
}

func generateProvinceBasedHeightmapThresholdMap() error {
	fmt.Println("Generating province-based heightmap threshold map...")

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

			max := 0
			var heightmapColor color.RGBA
			for c, i := range heightmapColors {
				if i > max {
					max = i
					heightmapColor = c
				}
			}

			if heightmapColor.R > 222 {
				for _, pc := range p.PixelCoords {
					img.Set(pc.X, pc.Y, color.RGBA{255, 0, 255, 255})
				}
			} else {
				for _, pc := range p.PixelCoords {
					img.Set(pc.X, pc.Y, color.RGBA{heightmapColor.R, heightmapColor.G, heightmapColor.B, heightmapColor.A})
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
	fmt.Println("Saved 'province_based_heightmap_threshold.png'")

	return nil
}
