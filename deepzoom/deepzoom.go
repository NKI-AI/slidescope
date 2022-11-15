package deepzoom

import (
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/NKI-AI/openslide-go/openslide"
	log "github.com/sirupsen/logrus"
	"golang.org/x/image/draw"
	"image"
	"image/color"
	"math"
	"strconv"
	"time"
)

type DeepZoom struct {
	tileCount           int      // Number of tiles in pyramid
	LevelDimensions     [][2]int // The dimensions of the levels in the active area (not necessarily those in the pyramidal image)
	zDimensions         [][2]int
	levelTiles          [][2]int
	levelCount          int
	dzLevelToSlideLevel []int // List which maps the index of the dz level to the level in the underlying image
	lzDownsamples       []float64
	level0Offset        [2]int
	tileSize            int              // Tile size of the resulting pyramid
	tileOverlap         int              // Amount the tiles should overlap
	Format              string           // jpeg or png
	bgColor             color.Color      // The background color in case the alpha channel is zero
	Slide               *openslide.Slide // Reference to the Slide
}

type TileInfo struct {
	level0Location  [2]int
	slideLevel      int
	levelOutputSize [2]int
	outputTileSize  [2]int
}

// CreateAssociatedDeepZoom Create DeepZoom for the associated images
func CreateAssociatedDeepZoom(
	slide openslide.Slide,
	associatedName string,
	tileSize int,
	tileOverlap int,
	format string) (DeepZoom, error) {

	dimensions := slide.AssociatedImageDimensions()[associatedName]
	var levelDimensions [][2]int
	levelDimensions = append(levelDimensions, dimensions)
	dz, err := createDeepZoom(slide, tileSize, tileOverlap, [2]int{0, 0}, levelDimensions, levelDimensions[0], format)
	return dz, err
}

// CreateDeepZoom Create DeepZoom object
func CreateDeepZoom(slide openslide.Slide, tileSize int, tileOverlap int, respectLimits bool, format string) (DeepZoom, error) {
	// Parse offsets
	var level0Offset [2]int
	var levelDimensions [][2]int
	if !respectLimits {
		level0Offset = [2]int{0, 0}
		for i := 0; i < slide.LevelCount(); i++ {
			dims := slide.LevelDimensions(i)
			levelDimensions = append(levelDimensions, [2]int{dims[0], dims[1]})
		}

	} else {
		var sizeScale [2]float64
		_boundsOffsetX := slide.PropertyValue(openslide.PropBoundsX)
		_boundsOffsetY := slide.PropertyValue(openslide.PropBoundsY)

		if _boundsOffsetX == "" || _boundsOffsetY == "" {
			if _boundsOffsetX == "" {
				level0Offset[0] = 0
			}
			if _boundsOffsetY == "" {
				level0Offset[1] = 0
			}
		} else {
			boundsOffsetX, err := strconv.ParseInt(_boundsOffsetX, 10, 64)
			if err != nil {
				level0Offset[0] = 0
			} else {
				level0Offset[0] = int(boundsOffsetX)
			}
			boundsOffsetY, _ := strconv.ParseInt(_boundsOffsetY, 10, 64)
			if err != nil {
				level0Offset[1] = 0
			} else {
				level0Offset[1] = int(boundsOffsetY)
			}
		}

		_boundsWidth := slide.PropertyValue(openslide.PropBoundsWidth)
		_boundsHeight := slide.PropertyValue(openslide.PropBoundsHeight)
		if _boundsWidth == "" || _boundsHeight == "" {
			if _boundsWidth == "" {
				sizeScale[0] = 1.0
			}

			if _boundsHeight == "" {
				sizeScale[1] = 1.0
			}
		} else {
			boundsWidth, err := strconv.ParseFloat(_boundsWidth, 64)
			if err != nil {
				sizeScale[0] = 1.0
			} else {
				sizeScale[0] = boundsWidth / float64(slide.LargestLevelDimensions()[0])
			}
			boundsHeight, err := strconv.ParseFloat(_boundsHeight, 64)
			if err != nil {
				sizeScale[1] = 1.0
			} else {
				sizeScale[1] = boundsHeight / float64(slide.LargestLevelDimensions()[1])
			}
		}

		// Dimensions of the active area
		for i := 0; i < slide.LevelCount(); i++ {
			levelDimension := slide.LevelDimensions(i)

			a := math.Ceil(float64(levelDimension[0]) * sizeScale[0])
			b := math.Ceil(float64(levelDimension[1]) * sizeScale[1])
			levelDimensions = append(levelDimensions, [2]int{int(a), int(b)})
		}
	}

	dz, err := createDeepZoom(slide, tileSize, tileOverlap, level0Offset, levelDimensions, slide.LargestLevelDimensions(), format)
	return dz, err
}

// GetCachedDeepZoom Get DeepZoom object from cache
func GetCachedDeepZoom(cache *LocalCache, imageIdentifier string, imagePath string, tileSize int, tileOverlap int, respectLimits bool, format string) (*DeepZoom, error) {
	var cacheDeepZoom NamedDeepZoom
	cacheDeepZoom, err := cache.Read(imageIdentifier)
	if err != nil {
		// create the deepZoom in cache
		log.Info(fmt.Sprintf("Not in cache, will add: %s", imageIdentifier))
		slide, err := openslide.Open(imagePath)
		if err != nil {
			return nil, errors.New(err.Error())
		}
		deepZoom, err := CreateDeepZoom(slide, tileSize, tileOverlap, respectLimits, format)
		if err != nil {
			return nil, errors.New(err.Error())
		}
		// Caching the DeepZoom
		cacheDeepZoom = NamedDeepZoom{
			Id:       imageIdentifier,
			DeepZoom: &deepZoom,
		}
		err = cache.Update(cacheDeepZoom, time.Now().Unix()+500)
		if err != nil {
			return nil, errors.New(err.Error())
		}

	}
	deepZoom := cacheDeepZoom.DeepZoom
	return deepZoom, nil
}

// createDeepZoom Helper function to create DeepZoom objects
func createDeepZoom(
	slide openslide.Slide,
	tileSize int,
	tileOverlap int,
	level0Offset [2]int,
	levelDimensions [][2]int,
	level0Dimensions [2]int,
	format string) (DeepZoom, error) {

	var zDimensions [][2]int
	zSize := level0Dimensions
	zDimensions = append(zDimensions, zSize)

	for zSize[0] > 1 || zSize[1] > 1 {
		for i, _ := range zSize {
			zSize[i] = int(math.Max(1.0, math.Ceil(float64(zSize[i])/2.0)))
		}
		zDimensions = append(zDimensions, zSize)
	}
	// reverse zDimensions
	for i, j := 0, len(zDimensions)-1; i < j; i, j = i+1, j-1 {
		zDimensions[i], zDimensions[j] = zDimensions[j], zDimensions[i]
	}

	var levelTiles [][2]int
	for _, zDim := range zDimensions {
		var currTuple [2]int
		for i := 0; i < 2; i++ {
			currTuple[i] = int(math.Ceil(float64(zDim[i]) / float64(tileSize)))
		}
		levelTiles = append(levelTiles, currTuple)
	}

	levelCount := len(zDimensions)

	var level0zDownsamples []float64
	for i := 0; i < levelCount; i++ {
		level0zDownsamples = append(level0zDownsamples, math.Pow(2, float64(levelCount-i-1)))
	}

	var dzLevelToSlideLevel []int
	for _, downSample := range level0zDownsamples {
		dzLevelToSlideLevel = append(dzLevelToSlideLevel, slide.BestLevelForDownsample(downSample))
	}

	var lzDownsamples []float64
	for i := 0; i < levelCount; i++ {
		lzDownsamples = append(lzDownsamples, level0zDownsamples[i]/slide.LevelDownsamples()[dzLevelToSlideLevel[i]])
	}

	var _bgColor = slide.PropertyValue(openslide.PropBackgroundColor)
	if _bgColor == "" {
		_bgColor = "ffffff"
	}
	// TODO: handle error
	bgColor, _ := Hex2Color(Hex(_bgColor))

	var tileCount = 0
	for _, levelTile := range levelTiles {
		tileCount += levelTile[0] * levelTile[1]
	}

	return DeepZoom{
		tileCount:           tileCount,  // Number of tiles in the complete pyramid
		levelCount:          levelCount, // Number of levels in the pyramid
		levelTiles:          levelTiles, // Grid size of tiles at level i
		zDimensions:         zDimensions,
		LevelDimensions:     levelDimensions,
		dzLevelToSlideLevel: dzLevelToSlideLevel,
		lzDownsamples:       lzDownsamples,
		level0Offset:        level0Offset,
		tileSize:            tileSize,
		tileOverlap:         tileOverlap,
		Format:              format,
		bgColor:             bgColor,
		Slide:               &slide,
	}, nil
}

type Hex string

// Hex2Color Convert Hex-html colors to color.Color's.
// For instance `ffffff` returns white.
func Hex2Color(hex Hex) (color.Color, error) {
	type RGB struct {
		Red   uint8
		Green uint8
		Blue  uint8
	}

	var rgb RGB
	values, err := strconv.ParseUint(string(hex), 16, 32)

	if err != nil {
		return color.RGBA{}, errors.New("cannot parse RGB values " + string(hex))
	}

	rgb = RGB{
		Red:   uint8(values >> 16),
		Green: uint8((values >> 8) & 0xFF),
		Blue:  uint8(values & 0xFF),
	}
	outputColor := color.RGBA{R: rgb.Red, G: rgb.Green, B: rgb.Blue, A: 0xff}

	return outputColor, nil
}

type DziSize struct {
	Width  int `xml:"Width,attr"`
	Height int `xml:"Height,attr"`
}

type DziImage struct {
	XMLName  xml.Name `xml:"Image"`
	Xmlns    string   `xml:"xmlns,attr"`
	TileSize int      `xml:"TileSize,attr"`
	Overlap  int      `xml:"Overlap,attr"`
	Format   string   `xml:"Format,attr"`
	Size     DziSize
}

// _getDzi Helper function to create DeepZoom XMLs
func (deepZoom DeepZoom) _getDzi(dimensions [2]int) (*DziImage, error) {
	if deepZoom.Format != "jpeg" && deepZoom.Format != "png" {
		return nil, errors.New("only allowed formats are jpeg or png")
	}
	v := &DziImage{
		Xmlns:    "http://schemas.microsoft.com/deepzoom/2008",
		TileSize: deepZoom.tileSize,
		Overlap:  deepZoom.tileOverlap,
		Format:   deepZoom.Format,
	}
	v.Size = DziSize{Width: dimensions[0], Height: dimensions[1]}

	return v, nil
}

// GetDzi Create DeepZoom XML
func (deepZoom DeepZoom) GetDzi() (*DziImage, error) {
	v, err := deepZoom._getDzi(deepZoom.Slide.LargestLevelDimensions())
	if err != nil {
		return &DziImage{}, errors.New(err.Error())
	}
	return v, nil
}

// GetAssociatedDzi Create DeepZoom XML for the associated images.
func (deepZoom DeepZoom) GetAssociatedDzi(associatedName string) (*DziImage, error) {
	dimensions, ok := deepZoom.Slide.AssociatedImageDimensions()[associatedName]
	if !ok {
		return nil, errors.New("associated image does not exist")
	}
	v, _ := deepZoom._getDzi(dimensions)
	return v, nil
}

// rescaleIfNeeded Given a tile, and the tileInfo will rescale the image of the tileInfo suggests so
func rescaleIfNeeded(tile image.Image, tileInfo TileInfo) image.Image {
	bounds := tile.Bounds()
	newTile := image.NewRGBA(bounds)
	draw.Draw(newTile, bounds, tile, bounds.Min, draw.Src)

	if tileInfo.levelOutputSize[0] != tileInfo.outputTileSize[0] || tileInfo.levelOutputSize[1] != tileInfo.outputTileSize[1] {
		outputImage := image.NewRGBA(image.Rect(0, 0, tileInfo.outputTileSize[0], tileInfo.outputTileSize[1]))
		draw.BiLinear.Scale(outputImage, outputImage.Bounds(), tile, tile.Bounds(), draw.Over, nil)
		newTile = outputImage
	}
	return newTile
}

// GetAssociatedTile Get a DeepZoom tile for an associated image
func (deepZoom DeepZoom) GetAssociatedTile(associatedName string, dzLevel int, tLocation [2]int) (image.Image, error) {
	if tLocation[0] < 0 || tLocation[1] < 0 {
		return nil, errors.New("negative locations are not supported")
	}

	associatedImage, _ := deepZoom.Slide.ReadAssociatedImage(associatedName)

	associatedDeepZoom, _ := CreateAssociatedDeepZoom(
		*deepZoom.Slide,
		associatedName,
		deepZoom.tileSize,
		deepZoom.tileOverlap,
		deepZoom.Format,
	)
	tileInfo, _ := associatedDeepZoom.getTileInfo(dzLevel, tLocation)

	dimensions := deepZoom.Slide.AssociatedImageDimensions()[associatedName]
	width := dimensions[0]
	height := dimensions[1]

	// Now we need to translate the level to the coordinates

	tile := image.NewRGBA(image.Rect(0, 0, tileInfo.outputTileSize[0], tileInfo.outputTileSize[0]))
	location := tileInfo.level0Location

	var X int
	var Y int
	for x := 0; x < tileInfo.outputTileSize[0]; x++ {
		for y := 0; y < tileInfo.outputTileSize[0]; y++ {
			// new coordinates
			X = x + location[0]
			Y = y + location[1]

			if X >= location[0]+width {
				break
			}

			if Y >= location[1]+height {
				break
			}
			pixel := associatedImage.At(X, Y)
			tile.Set(x, y, pixel)
		}
	}

	newTile := rescaleIfNeeded(tile, tileInfo)

	return newTile, nil
}

// GetTile Return a DeepZoom tile
func (deepZoom DeepZoom) GetTile(dzLevel int, location [2]int) (image.Image, error) {
	tileInfo, err := deepZoom.getTileInfo(dzLevel, location)
	if err != nil {
		return nil, err
	}

	var tile image.Image
	tile, err = deepZoom.Slide.ReadRegion(
		tileInfo.level0Location[0],
		tileInfo.level0Location[1],
		tileInfo.slideLevel,
		tileInfo.levelOutputSize[0],
		tileInfo.levelOutputSize[1],
	)

	if err != nil {
		return nil, errors.New(err.Error())
	}

	// When the alpha channel is zero, set colors to bgColor
	// Create a copy of this image, so the underlying bytes can be changed
	bounds := tile.Bounds()
	newTile := image.NewRGBA(bounds)
	draw.Draw(newTile, bounds, tile, bounds.Min, draw.Src)

	for x := 0; x < tileInfo.levelOutputSize[0]; x++ {
		for y := 0; y < tileInfo.levelOutputSize[1]; y++ {
			_, _, _, a := newTile.At(x, y).RGBA()
			if a != 0 && a != 65535 {
				return nil, errors.New("can only map 65535 or a")
			}
			if a == 0 {
				newTile.Set(x, y, deepZoom.bgColor)
			}
		}
	}

	output := rescaleIfNeeded(newTile, tileInfo)

	return output, err
}

// getTileInfo Return information requires to generate a DeepZoom tile
func (deepZoom DeepZoom) getTileInfo(dzLevel int, tLocation [2]int) (TileInfo, error) {
	if dzLevel < 0 || dzLevel > deepZoom.levelCount {
		log.Info("Invalid level")
		return TileInfo{}, errors.New("invalid level")
	}

	for i, t := range tLocation {
		tLim := deepZoom.levelTiles[dzLevel]
		if t < 0 || t >= tLim[i] {
			return TileInfo{}, errors.New("invalid address")
		}
	}

	// Get preferred slide level
	slideLevel := deepZoom.dzLevelToSlideLevel[dzLevel]

	// Calculate top/left and bottom/right overlap
	var zOverlapTl [2]int
	var zOverlapBr [2]int
	for i := 0; i < 2; i++ {
		if tLocation[i] != 0 {
			zOverlapTl[i] = deepZoom.tileOverlap
		} else {
			zOverlapTl[i] = 0
		}

		if tLocation[i] != deepZoom.levelTiles[dzLevel][i]-1 {
			zOverlapBr[i] = deepZoom.tileOverlap
		} else {
			zOverlapBr[i] = 0
		}
	}

	// Get final size of the tile
	var outputTileSize [2]int
	for i := 0; i < 2; i++ {
		t := tLocation[i]
		zLim := deepZoom.zDimensions[dzLevel][i]
		zTl := zOverlapTl[i]
		zBr := zOverlapBr[i]

		values := []int{deepZoom.tileSize, zLim - deepZoom.tileSize*t}

		min := values[0]
		for _, v := range values {
			if v < min {
				min = v
			}
		}
		outputTileSize[i] = min + zTl + zBr
	}

	// Obtain the region coordinates
	zLocation := [2]int{deepZoom.tileSize * tLocation[0], deepZoom.tileSize * tLocation[1]}
	var0 := deepZoom.lzDownsamples[dzLevel] * float64(zLocation[0]-zOverlapTl[0])
	var1 := deepZoom.lzDownsamples[dzLevel] * float64(zLocation[1]-zOverlapTl[1])

	lLocation := [2]float64{var0, var1}
	var level0Location [2]int

	// Round location down and size up, and add offset of active area
	level0Location[0] = deepZoom.level0Offset[0] + int(deepZoom.Slide.LevelDownsample(slideLevel)*lLocation[0])
	level0Location[1] = deepZoom.level0Offset[1] + int(deepZoom.Slide.LevelDownsample(slideLevel)*lLocation[1])

	var levelOutputSize [2]int
	for i := 0; i < 2; i++ {
		lLim := deepZoom.LevelDimensions[slideLevel][i]
		levelOutputSize[i] = int(math.Min(
			math.Ceil(deepZoom.lzDownsamples[dzLevel]*float64(outputTileSize[i])),
			float64(lLim)-math.Ceil(lLocation[i]),
		),
		)

	}

	output := TileInfo{
		level0Location:  level0Location,  // Location express in level 0 coordinates
		slideLevel:      slideLevel,      // The level in the pyramid
		levelOutputSize: levelOutputSize, // The output size at the requested level
		outputTileSize:  outputTileSize,  // The size the final tile should be resized to
	}
	return output, nil
}
