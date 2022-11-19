package main

import "C"
import (
	"context"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	uuid "github.com/twinj/uuid"
	"net/http"
	"os"
	"os/signal"
	"slidescope/controllers"
	"slidescope/deepzoom"
	"slidescope/models"
	"slidescope/utils"
	"syscall"
	"time"
)

// CorsMiddleware Use middleware for CORS (Cross-Origin Resource Sharing)
// TODO: This is too broad, cannot expose to the internet!
// TODO: Allow to get the origins from the database or YAML/env?
// Use middleware
// CORS for * origins, allowing:
// - PUT, GET, POST and PATCH methods
// - Origin header
// - Credentials share
// - Preflight requests cached for 12 hours
func corsMiddleware() gin.HandlerFunc {
	_corsMiddleware := cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"PUT", "GET", "POST", "PATCH"},
		AllowHeaders:     []string{"Origin"},
		ExposeHeaders:    []string{"X-Requested-With, Content-Type, Origin, Authorization, Accept, Client-Security-Token, Accept-Encoding, x-access-token"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
	return _corsMiddleware
}

// RequestIDMiddleware Generate a UUID and attach it to each request
func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		_uuid := uuid.NewV4()
		c.Writer.Header().Set("X-Request-Id", _uuid.String())
		c.Next()
	}
}

func main() {
	log.Info("Starting SlideScope...")

	// Generate our config based on the config supplied
	// by the user in the flags
	configPath, debugMode, err := utils.ParseFlags()
	if err != nil {
		log.Fatal(err)
	}
	config, err := utils.NewConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	// Debug mode enables gin-gonic debug mode
	if debugMode == false {
		gin.SetMode(gin.ReleaseMode)
	}

	// Connect to the database
	models.ConnectDataBase(config.Sqlite.Filename)

	r := gin.Default()

	r.Use(corsMiddleware())
	r.Use(requestIDMiddleware())
	r.Use(gzip.Gzip(gzip.DefaultCompression))

	// Version tag to test against
	r.GET("/version", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "v0.0.1",
		})
	})

	// REST API to create images
	// Currently no authentication is used
	api := r.Group("/api")
	v1 := api.Group("/v1")
	{
		v1.GET("/images", controllers.FindImages)
		v1.POST("/images", controllers.CreateImage)
		v1.GET("/images/:id", controllers.FindImage)
		v1.PATCH("/images/:id", controllers.UpdateImage)
		v1.DELETE("/images/:id", controllers.DeleteImage)
		// Route to return openslide properties
		api.GET("/images/:id/properties")
	}

	// Create a cache for the deepzoom objects
	cache := deepzoom.NewLocalCache(10e8)

	// Routes that generate the deepzoom pyramid
	// These pyramids are cached and released once in a while.
	// TODO: DDOS is possible by opening a lot of images (if they are in the database)
	// TODO: To alleviate this, a check on cache size should be done and a "server busy" response should be issued.
	dzRoutes := r.Group("/deepzoom")
	{
		dzRoutes.GET("/:image_identifier/slide_files/:level/:location", controllers.GetTile(cache, config))

		// TODO: GetOverlayTile
		dzRoutes.GET("/:image_identifier/overlays/:overlay_identifier/slide_files/:level/:location", controllers.GetOverlayTile(cache, config))
		dzRoutes.GET("/:image_identifier/slide.dzi", controllers.GetDzi(cache, config))

		// TODO: Create GetOverlayDzi, or merge GetOverlayTile with GetTile
		dzRoutes.GET("/:image_identifier/overlays/:overlay_identifier/slide.dzi", controllers.GetDzi(cache, config))

		// Thumbnail routes
		dzRoutes.GET("/:image_identifier/thumbnail.jpg", controllers.GetThumbnail(cache, config))
		dzRoutes.GET("/:image_identifier/thumbnail.png", controllers.GetThumbnail(cache, config))

		// TODO: pass a parameter ?all=true to pass the full map, otherwise just shape and mpp is relevant
		dzRoutes.GET("/:image_identifier/properties", controllers.GetImageProperties(cache, config))
	}

	r.LoadHTMLGlob("frontend/templates/**/*.tmpl")
	r.Static("/static", "frontend/static")

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"title": "Main website",
		})
	})

	r.GET("/viewer", func(c *gin.Context) {
		c.HTML(http.StatusOK, "viewer.tmpl", gin.H{
			"title": "Viewer",
		})
	})

	//// Register an login controllers
	//api.POST("/register", controllers.Register)
	//api.POST("/login", controllers.Login)
	//
	//// TODO: Use https://github.com/gin-contrib/sessions to keep the cookie
	//protected := r.Group("/api/admin")
	//protected.Use(middlewares.JwtAuthMiddleware())
	//protected.GET("/user", controllers.CurrentUser)
	//

	addr := fmt.Sprintf(":%s", config.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal)
	// kill (no param) default send syscanll.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall. SIGKILL but can"t be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}
	// catching ctx.Done(). timeout of 1 seconds.
	select {
	case <-ctx.Done():
		log.Info("Timeout of 1 seconds.")
	}

	//log.Info("Emptying deepzoom cache...")
	//cache.EmptyCache()

	log.Info("Server exiting")

}
