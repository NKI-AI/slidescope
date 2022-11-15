package controllers

import (
	"fmt"
	"github.com/NKI-AI/openslide-go/openslide"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
	"slidescope/models"
)

// FindImages Find all images with annotations
func FindImages(c *gin.Context) {
	var images []models.Image
	models.DB.Preload("MaskAnnotations").Find(&images)

	c.JSON(http.StatusOK, gin.H{"data": images})
}

type CreateImageInput struct {
	Path            string                  `json:"path" binding:"required"`
	Identifier      string                  `json:"identifier" binding:"required"`
	MaskAnnotations []models.MaskAnnotation `json:"mask_annotations"`
}

// CreateImage Create a new image
func CreateImage(c *gin.Context) {
	// Validate input
	var input CreateImageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vendor, err := openslide.DetectVendor(input.Path)
	if err != nil {
		log.Println(fmt.Sprintf("Cannot detect vendor for slide %s", input.Path))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	log.Info(fmt.Sprintf("Importing %s with vendor %s", input.Path, vendor))

	if input.MaskAnnotations != nil {
		for _, maskAnnotation := range input.MaskAnnotations {
			vendor, err := openslide.DetectVendor(maskAnnotation.Path)
			log.Info(fmt.Sprintf("Importing mask %s with vendor %s", maskAnnotation.Path, vendor))
			if err != nil {
				log.Info(fmt.Sprintf("Cannot detect vendor for mask %s", maskAnnotation.Path))
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

		}
	}

	// Create image
	image := models.Image{Path: input.Path, Identifier: input.Identifier, MaskAnnotations: input.MaskAnnotations}
	models.DB.Create(&image)

	c.JSON(http.StatusOK, gin.H{"data": image})
}

// FindImage Find an image
func FindImage(c *gin.Context) { // Get model if exist
	var image models.Image

	if err := models.DB.Preload("MaskAnnotations").Where("id = ?", c.Param("id")).First(&image).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Record not found!"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": image})
}

type UpdateImageInput struct {
	Path            string                  `json:"path"`
	Identifier      string                  `json:"identifier"`
	MaskAnnotations []models.MaskAnnotation `json:"mask_annotations"`
}

// UpdateImage Update an image
func UpdateImage(c *gin.Context) {
	// Get model if exist
	var image models.Image
	if err := models.DB.Where("id = ?", c.Param("id")).First(&image).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Record not found!"})
		return
	}

	// Validate input
	var input UpdateImageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
}

// DeleteImage Delete an image
func DeleteImage(c *gin.Context) {
	// Get model if exist
	var image models.Image
	if err := models.DB.Where("id = ?", c.Param("id")).First(&image).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Record not found!"})
		return
	}

	models.DB.Delete(&image)

	c.JSON(http.StatusOK, gin.H{"data": true})
}
