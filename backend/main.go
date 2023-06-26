package main

import (
	"streamfox-backend/controllers"
	"streamfox-backend/middleware"
	"streamfox-backend/models"

	"github.com/gin-gonic/gin"
)

func main() {
	models.Setup()

	router := gin.Default()

	auth := router.Group("/auth")
	auth.POST("/register", controllers.Register)
	auth.POST("/login", controllers.Login)

	api := router.Group("/api")
	api.Use(middleware.JwtAuthMiddleware())
	api.GET("/user", controllers.GetUser)

	router.Run(":5000")
}
