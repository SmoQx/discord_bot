package main

import (
	"database/sql"
	"discord_bot/crud"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetSongs(ctx *gin.Context, db *sql.DB) {
	songs, err := crud.GetSongs(db)

	if err != nil {
		ctx.JSON(http.StatusNotFound, nil)
	}

	ctx.JSON(http.StatusOK, songs)
}

func GetPlaylists(ctx *gin.Context, db *sql.DB) {
	playlists, _ := crud.GetPlayLists(db)

	fmt.Println(playlists)

	// if err != nil {
	// 	ctx.JSON(http.StatusNotFound, nil)
	// }

	ctx.JSON(http.StatusOK, playlists)
}

func RunServer(db *sql.DB) {

	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, PUT, POST, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	router.Static("/static", "./static")
	router.LoadHTMLGlob("templates/*")

	router.GET("/", func(ctx *gin.Context) {
		ctx.HTML(http.StatusOK, "discord-music-bot.html", nil)
	})

	router.GET("/api/songs", func(ctx *gin.Context) {
		GetSongs(ctx, db)
	})

	router.GET("/api/playlists", func(ctx *gin.Context) {
		GetPlaylists(ctx, db)
	})

	router.Run("127.0.0.1:8080")
}
