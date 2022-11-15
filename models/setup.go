package models

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"os"

	"github.com/joho/godotenv"
	_ "gorm.io/driver/sqlite" // Sqlite driver based on GGO
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDataBase() {
	err := godotenv.Load(".env.example")

	if err != nil {
		log.Fatalf("Error loading .env.example file")
	}

	DbName := os.Getenv("DB_NAME")
	DbUrl := DbName + ".sqlite"
	DB, err = gorm.Open(sqlite.Open(DbUrl), &gorm.Config{})

	if err != nil {
		log.Fatal(fmt.Sprintf("Cannot connect sqlite database at %s", DbUrl))
		log.Fatal("connection error:", err)
	} else {
		log.Info(fmt.Sprintf("Connecting sqlite database at %s", DbUrl))
	}

	DB.AutoMigrate(&User{})
	DB.AutoMigrate(&Image{})
	DB.AutoMigrate(&MaskAnnotation{})

}
