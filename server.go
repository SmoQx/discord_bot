package main

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetSongs(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"test": "test",
	})
}

func RunServer(db *sql.DB) {

	router := gin.Default()

	router.Static("/static", "./static")

	router.GET("/", func(ctx *gin.Context) {
		ctx.HTML(http.StatusOK, "discord-music-bot.html", nil)
	})

	router.GET("/get_songs", GetSongs)

	router.Run("127.0.0.1:8080")
}
