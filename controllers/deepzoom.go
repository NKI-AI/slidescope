package controllers

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"image"
	"image/jpeg"
	"net/http"
	"slidescope/deepzoom"
	"slidescope/models"
	"slidescope/utils"
	"strconv"
	"strings"
)

type DeepZoomCoordinates struct {
	contentType string
	level       int
	location    [2]int
}

type DeepZoomOutput struct {
	Image           *models.Image
	MaskAnnotations *models.MaskAnnotation
}

// parseDeepZoomCoordinates Parse the coordinates from the context and return as a DeepZoomCoordinates object
func parseDeepZoomCoordinates(c *gin.Context) (DeepZoomCoordinates, error) {
	level := c.Param("level")
	location := c.Param("location")
	s := strings.Split(location, ".")

	if s[1] != "png" && s[1] != "jpg" {
		return DeepZoomCoordinates{}, errors.New("only jpg or png is allowed as an extension")
	}

	var contentType = "image/" + s[1]

	s = strings.Split(s[0], "_")
	row := s[1]
	column := s[0]

	levelInt, err := strconv.ParseInt(level, 10, 64)

	rowInt, err := strconv.ParseInt(row, 10, 64)
	if err != nil {
		return DeepZoomCoordinates{}, errors.New("cannot parse row")
	}
	columnInt, err := strconv.ParseInt(column, 10, 64)
	if err != nil {
		return DeepZoomCoordinates{}, errors.New("cannot parse column")
	}

	return DeepZoomCoordinates{
		contentType: contentType,
		level:       int(levelInt),
		location:    [2]int{int(columnInt), int(rowInt)},
	}, nil
}

// parseIdentifier Parse the image identifier to a database entry.
func parseIdentifier(c *gin.Context) (models.Image, error) {
	imageId := c.Param("image_identifier")
	// Let's try to find it.
	var reqImage models.Image

	if err := models.Database.Where("Identifier = ?", imageId).First(&reqImage).Error; err != nil {
		return models.Image{}, errors.New("image not found")
	}
	return reqImage, nil
}

// writeTileToAPI Write the tile to the API output.
func writeTileToAPI(c *gin.Context, header *http.Header, w gin.ResponseWriter, contentType string, tile image.Image) {
	var tileBuffer *[]byte
	var err error
	if contentType == "image/jpeg" {
		tileBuffer, err = utils.ImageToJpgBuffer(tile, &jpeg.Options{Quality: 75})
	} else { // PNG
		tileBuffer, err = utils.ImageToPngBuffer(tile)
	}

	if err != nil {
		log.Warn(fmt.Sprintf("Error writing tile with content type %s to image buffer: %s", contentType, err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"data": err.Error()})
		return
	}
	_, err = w.Write(*tileBuffer)

	if err != nil {
		log.Warn(fmt.Sprintf("Error writing tile with content type %s to image buffer: %s", contentType, err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"data": err.Error()})
		return
	}
	header.Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	w.(http.Flusher).Flush()
}

// writeTileFromCachedDeepZoom Write the tile to the output from a cached deepzoom object.
func writeTileFromCachedDeepZoom(c *gin.Context, cache *deepzoom.LocalCache, Identifier string, Path string, tileSize int, tileOverlap int) {
	var deepZoom *deepzoom.DeepZoom

	deepZoom, err := deepzoom.GetCachedDeepZoom(cache, Identifier, Path, tileSize, tileOverlap, true, "png")

	if err != nil {
		log.Warn(fmt.Sprintf("Error getting cached deep zoom with identifier %s and path %s: %s", Identifier, Path, err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"data": err.Error()})
		return
	}
	var coordinates DeepZoomCoordinates
	coordinates, err = parseDeepZoomCoordinates(c)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"data": err.Error()})
	}

	var tile image.Image
	var level = coordinates.level
	var location = coordinates.location

	tile, err = deepZoom.GetTile(level, location)

	if err != nil {
		log.Warn(fmt.Sprintf("Error getting deep zoom tile with identifier %s and path %s: %s", Identifier, Path, err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"data": err.Error()})
	}

	w := c.Writer
	header := w.Header()
	writeTileToAPI(c, &header, w, coordinates.contentType, tile)
}

// GetOverlayTile Get a tile for an overlay
func GetOverlayTile(cache *deepzoom.LocalCache, config *utils.Config) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		imageId := c.Param("image_identifier")
		overlayId := c.Param("overlay_identifier")

		// Let's try to find it.
		var reqImage models.Image
		if err := models.Database.Preload("MaskAnnotations", "Identifier = ?", overlayId).Where("Identifier = ?", imageId).First(&reqImage).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		mask := reqImage.MaskAnnotations[0]
		writeTileFromCachedDeepZoom(
			c,
			cache,
			mask.Identifier,
			mask.Path,
			config.DeepZoom.TileSize,
			config.DeepZoom.TileOverlap)
	}
	return fn
}

// GetTile Get DeepZoom tile and write to output
func GetTile(cache *deepzoom.LocalCache, config *utils.Config) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		parsedIdentifier, err := parseIdentifier(c)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		writeTileFromCachedDeepZoom(
			c,
			cache,
			parsedIdentifier.Identifier,
			parsedIdentifier.Path,
			config.DeepZoom.TileSize,
			config.DeepZoom.TileOverlap)
	}
	return fn
}

// GetThumbnail Get the thumbnail of an image.
func GetThumbnail(cache *deepzoom.LocalCache, config *utils.Config) gin.HandlerFunc {
	// Format is ignored, as thumbnails are postfixed with png or jpg
	tileSize := config.DeepZoom.TileSize
	tileOverlap := config.DeepZoom.TileOverlap
	fn := func(c *gin.Context) {
		parsedIdentifier, err := parseIdentifier(c)
		// Only thumbnail.{jpg,png} is allowed in the route
		splitPath := strings.Split(c.FullPath(), ".")
		format := splitPath[len(splitPath)-1]
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		var size = c.DefaultQuery("size", "512")
		sizeInt, err := strconv.ParseInt(size, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Incorrect value for size."})
			return
		}

		if sizeInt > 1024 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Too large thumbnail requested."})
			return
		}
		var quality = c.DefaultQuery("Q", "-1")
		jpgQuality, err := strconv.ParseInt(quality, 10, 64)
		if format == "jpg" {
			if jpgQuality == -1 {
				jpgQuality = 75
			}

			if jpgQuality < 0 || jpgQuality > 100 {
				c.JSON(http.StatusBadRequest, gin.H{"message": "Incorrect value for quality."})
				return
			}
		}
		if format == "png" && jpgQuality != -1 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Compression quality only makes sense for jpg."})
		}

		deepZoom, err := deepzoom.GetCachedDeepZoom(
			cache,
			parsedIdentifier.Identifier,
			parsedIdentifier.Path,
			tileSize,
			tileOverlap, true, config.DeepZoom.Format)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"data": err.Error()})
			return
		}

		w := c.Writer
		header := w.Header()
		header.Set("Content-Type", "image/"+format)
		w.WriteHeader(http.StatusOK)
		thumbnail, err := deepZoom.Slide.GetThumbnail(int(sizeInt))
		var thumbnailBuf *[]byte
		if format == "jpeg" {
			thumbnailBuf, err = utils.ImageToJpgBuffer(thumbnail, &jpeg.Options{Quality: int(jpgQuality)})
		} else {
			thumbnailBuf, err = utils.ImageToPngBuffer(thumbnail)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"data": err.Error()})
			return
		}
		_, err = w.Write(*thumbnailBuf)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"data": err.Error()})
			return
		}

		w.(http.Flusher).Flush()
	}
	return fn
}

// GetDzi Get the deepzoom XML for a given image
func GetDzi(cache *deepzoom.LocalCache, config *utils.Config) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		parsedIdentifier, err := parseIdentifier(c)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		var deepZoom *deepzoom.DeepZoom

		deepZoom, err = deepzoom.GetCachedDeepZoom(
			cache,
			parsedIdentifier.Identifier,
			parsedIdentifier.Path,
			config.DeepZoom.TileSize,
			config.DeepZoom.TileOverlap,
			true,
			config.DeepZoom.Format)

		message, err := deepZoom.GetDzi()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"data": err.Error()})
			return
		}
		c.XML(200, &message)
	}
	return fn
}

// GetImageProperties Get all the properties given in the OpenSlide object
func GetImageProperties(cache *deepzoom.LocalCache, config *utils.Config) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		tileSize := config.DeepZoom.TileSize
		tileOverlap := config.DeepZoom.TileOverlap
		format := config.DeepZoom.Format
		parsedIdentifier, err := parseIdentifier(c)

		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		deepZoom, err := deepzoom.GetCachedDeepZoom(cache, parsedIdentifier.Identifier, parsedIdentifier.Path, tileSize, tileOverlap, true, format)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"data": err.Error()})
			return
		}
		properties := deepZoom.Slide.Properties()
		c.IndentedJSON(http.StatusOK, &properties)

	}
	return fn
}
