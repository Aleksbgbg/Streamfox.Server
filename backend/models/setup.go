package models

import (
	"fmt"
	"log"
	"streamfox-backend/utils"

	"github.com/bwmarrin/snowflake"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DATABASE *gorm.DB
var ID_GENERATOR *snowflake.Node

func Setup() {
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatal("Error loading .env file.")
	}

	DATABASE, err = gorm.Open(
		postgres.Open(fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Europe/London",
			utils.GetEnvVar(utils.DB_HOST),
			utils.GetEnvVar(utils.DB_USER),
			utils.GetEnvVar(utils.DB_PASSWORD),
			utils.GetEnvVar(utils.DB_NAME),
			utils.GetEnvVar(utils.DB_PORT),
		)),
		&gorm.Config{},
	)

	if err != nil {
		log.Fatal("Could not connect to database:", err)
	}

	DATABASE.AutoMigrate(&User{})
	DATABASE.AutoMigrate(&Video{})

	ID_GENERATOR, err = snowflake.NewNode(1)

	if err != nil {
		log.Fatal("Could not setup snowflake:", err)
	}
}
