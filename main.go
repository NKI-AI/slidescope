package main

import "C"
import (
	"flag"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
	"slidescope/controllers"
	"slidescope/deepzoom"
	"slidescope/middlewares"
	"slidescope/models"
	"time"
)

func main() {
	log.Info("Starting slidescope")
	tileSize := 254
	tileOverlap := 1

	// Debug mode enables gin-gonic debug mode
	debugMode := flag.Bool("debug", false, "GIN debug mode")
	if *debugMode == false {
		gin.SetMode(gin.ReleaseMode)
	}

	// Connect to the database
	models.ConnectDataBase()

	r := gin.Default()

	// CORS middleware to allow access from all URLs
	// TODO: This is too broad, cannot expose to the internet!
	// TODO: Allow to get the origins from the database or YAML/env?
	// Use middleware
	// CORS for * origins, allowing:
	// - PUT, GET, POST and PATCH methods
	// - Origin header
	// - Credentials share
	// - Preflight requests cached for 12 hours
	corsMiddleware := cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"PUT", "GET", "POST", "PATCH"},
		AllowHeaders:     []string{"Origin"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
	r.Use(corsMiddleware)

	// Version tag to test against
	r.GET("/version", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "v0.0.1",
		})
	})

	// REST api to create images
	// Currently no authentication is used
	api := r.Group("/api")
	api.GET("/images", controllers.FindImages)
	api.POST("/images", controllers.CreateImage)
	api.GET("/images/:id", controllers.FindImage)
	api.PATCH("/images/:id", controllers.UpdateImage)
	api.DELETE("/images/:id", controllers.DeleteImage)

	// Route to return openslide properties
	api.GET("/images/:id/properties")

	// Create a cache for the deepzoom routes
	cache := deepzoom.NewLocalCache(10e8)

	// Routes that generate the deepzoom pyramid
	// These pyramids are cached and released once in a while.
	// TODO: DDOS is possible by opening a lot of images (if they are in the database)
	// TODO: To alleviate this, a check on cache size should be done and a "server busy" response should be issued.
	// TODO: Replace the functions with controllers.
	deepzoomRoutes := r.Group("/deepzoom")
	deepzoomRoutes.GET("/:image_identifier/slide_files/:level/:location",
		controllers.GetTile(cache, tileSize, tileOverlap))

	// TODO: GetOverlayTile
	deepzoomRoutes.GET("/:image_identifier/overlays/:overlay_identifier/slide_files/:level/:location",
		controllers.GetOverlayTile(cache, tileSize, tileOverlap))

	deepzoomRoutes.GET("/:image_identifier/slide.dzi",
		controllers.GetDzi(cache, tileSize, tileOverlap, "png"))

	// TODO: Create GetOverlayDzi
	deepzoomRoutes.GET("/:image_identifier/overlays/:overlay_identifier/slide.dzi",
		controllers.GetDzi(cache, tileSize, tileOverlap, "png"))

	deepzoomRoutes.GET("/:image_identifier/thumbnail.jpg",
		controllers.GetThumbnail(cache, tileSize, tileOverlap, "jpeg"))
	deepzoomRoutes.GET("/:image_identifier/thumbnail.png",
		controllers.GetThumbnail(cache, tileSize, tileOverlap, "png"))
	// TODO: This is slightly awkward we need to pass tileSize and tileOverlap everywhere, even when it's not used
	// TODO: pass a parameter ?all=true to pass the full map, otherwise just shape and mpp is relevant
	deepzoomRoutes.GET("/:image_identifier/properties",
		controllers.GetImageProperties(cache, tileSize, tileOverlap, "png"))

	// Register an login controllers
	api.POST("/register", controllers.Register)
	api.POST("/login", controllers.Login)

	protected := r.Group("/api/admin")
	protected.Use(middlewares.JwtAuthMiddleware())
	protected.GET("/user", controllers.CurrentUser)

	r.Run() // listen and serve on 0.0.0.0:8080

	// TODO: How to make sure all file handlers are closed when exiting?
	cache.EmptyCache()

}
