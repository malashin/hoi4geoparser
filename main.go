package main

import (
	"bufio"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/image/bmp"
)

var modPath = "c:/Users/admin/Documents/Paradox Interactive/Hearts of Iron IV/mod/oldworldblues_border_wars"
var definitionsPath = modPath + "/map/definition.csv"
var adjacenciesPath = modPath + "/map/adjacencies.csv"
var provincesPath = modPath + "/map/provinces.bmp"
var statesPath = modPath + "/history/states"
var provincesIDMap = make(map[int]*Province)
var provincesRGBMap = make(map[color.Color]*Province)
var statesMap = make(map[int]*State)
var rState = regexp.MustCompile(`(?s:.*id.*=.*?(\d+).*name.*=.*\"(.+?)\".*provinces.*?=.*?{.*?(\d+.*?)\n.*?}.*})`)
var mapScalePixelToKm = 7.114

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

	// Write the output file.
	saveGeoData()

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
	PixelCoords  []image.Point
	CenterPoint  image.Point
	AdjacentTo   map[int]*Province
	ConnectedTo  map[int]*Province
	ImpassableTo map[int]*Province
}

// State represents an in-game state with all parsed data in it.
type State struct {
	ID           int
	Name         string
	IsCoastal    bool
	Continent    int
	PixelCoords  []image.Point
	CenterPoint  image.Point
	Provinces    map[int]*Province
	DistanceTo   map[int]int // Distance to other states.
	AdjacentTo   map[int]*State
	ConnectedTo  map[int]*State
	ImpassableTo map[int]*State
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
	a := strings.Split(s, ";")
	if len(a) != 10 {
		return errors.New("\"" + definitionsPath + "\": " + s + ": must contain 10 fields")
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

	// Parse each pixel in scanline order.
	for y := 0; y < provincesImage.Bounds().Max.Y; y++ {
		for x := 0; x < provincesImage.Bounds().Max.X; x++ {
			var e, s color.Color

			// Get the color of the current pixel.
			c := provincesImage.At(x, y)

			// Add pixel coordinates to the province that has this RGB value.
			provincesRGBMap[c].PixelCoords = append(provincesRGBMap[c].PixelCoords, image.Point{x, y})

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

func findCenterPoint(coords []image.Point) image.Point {
	// Find the bounding box of the province from its pixel coordinates.
	l := math.MaxInt64
	r := math.MinInt64
	t := math.MaxInt64
	b := math.MinInt64
	for _, c := range coords {
		if c.X < l {
			l = c.X
		}
		if c.X > r {
			r = c.X
		}
		if c.Y < t {
			t = c.Y
		}
		if c.Y > b {
			b = c.Y
		}
	}

	// Calculate the centerPoint of the bounding box.
	return image.Point{l + ((r - l) / 2), t + ((b - t) / 2)}
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
	sm := rState.FindStringSubmatch(s)
	sID, err := strconv.Atoi(sm[1])
	if err != nil {
		return state, err
	}

	state.ID = sID
	state.Name = sm[2]
	state.Continent = -1
	state.DistanceTo = make(map[int]int)
	state.Provinces = make(map[int]*Province)
	state.AdjacentTo = make(map[int]*State)
	state.ConnectedTo = make(map[int]*State)
	state.ImpassableTo = make(map[int]*State)
	provinces := strings.Split(strings.TrimSpace(sm[3]), " ")
	for _, p := range provinces {
		pID, err := strconv.Atoi(p)
		if err != nil {
			return state, err
		}
		state.Provinces[pID] = provincesIDMap[pID]
	}
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
			s1.PixelCoords = append(s1.PixelCoords, p1.PixelCoords...)

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
