package models

import "gorm.io/gorm"

type MaskAnnotation struct {
	gorm.Model
	ImageID    uint   `json:"image_id"`
	Path       string `json:"path"`
	Identifier string `json:"identifier"`
}
