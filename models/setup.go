package models

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	_ "gorm.io/driver/sqlite" // Sqlite driver based on GGO
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDataBase(dbUrl string) {
	DB, err := gorm.Open(sqlite.Open(dbUrl), &gorm.Config{})

	if err != nil {
		log.Fatal(fmt.Sprintf("Cannot connect sqlite database at %s", dbUrl))
		log.Fatal("connection error:", err)
	} else {
		log.Info(fmt.Sprintf("Connecting to sqlite database %s", dbUrl))
	}

	DB.AutoMigrate(&User{})
	DB.AutoMigrate(&Image{})
	DB.AutoMigrate(&MaskAnnotation{})

}
