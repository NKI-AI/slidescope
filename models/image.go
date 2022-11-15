package models

import "gorm.io/gorm"

type Image struct {
	gorm.Model
	ID              uint             `json:"id" gorm:"primary_key"`
	Path            string           `json:"path"`
	Identifier      string           `json:"identifier"`
	MaskAnnotations []MaskAnnotation `json:"mask_annotations" gorm:"foreignKey:ImageID"`
}
