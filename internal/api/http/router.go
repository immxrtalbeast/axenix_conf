package http

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func SetupRouter(roomController *RoomController, userController *UserController) *gin.Engine {
	router := gin.Default()
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{
		"http://localhost:3000",
		"http://138.124.14.255:3000",
		"https://138.124.14.255:3000",
		"https://138.124.14.255",
	}
	config.AllowCredentials = true
	config.AllowHeaders = []string{
		"Authorization",
		"Content-Type",
		"Origin",
		"Accept",
	}
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	config.ExposeHeaders = []string{"Set-Cookie"}
	router.Use(cors.New(config))
	router.GET("/healthz", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{"status": "ok"})
	})

	api := router.Group("/api")

	if userController != nil {
		users := api.Group("/users")
		users.POST("/create", userController.CreateUser)
		users.GET("/:userID", userController.GetUser)
	}

	if roomController != nil {
		rooms := api.Group("/rooms")
		rooms.POST("/create", roomController.CreateRoom)
		rooms.GET("/:roomID", roomController.GetRoom)
		rooms.GET("/link/:link", roomController.GetRoomByLink)
		rooms.GET("/:roomID/participants", roomController.ListParticipants)
		rooms.GET("/:roomID/ws", roomController.JoinRoom)
	}

	return router
}
