package models

import (
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var Database *gorm.DB

func ConnectDataBase(dbUrl string) {
	sqliteDb := sqlite.Open(dbUrl)
	var err error
	Database, err = gorm.Open(sqliteDb, &gorm.Config{})

	if err != nil {
		log.Fatal(fmt.Sprintf("Cannot connect sqlite database at %s", dbUrl))
		log.Fatal("connection error:", err)
	} else {
		log.Info(fmt.Sprintf("Connecting to sqlite database %s", dbUrl))
	}

	err = Database.AutoMigrate(&User{})
	err = Database.AutoMigrate(&Image{})
	err = Database.AutoMigrate(&MaskAnnotation{})

	if err != nil {
		log.Fatal(fmt.Sprintf("Cannot automigrate: %s"), err.Error())
	}

}
