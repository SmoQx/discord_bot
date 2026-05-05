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
	playlists, err := crud.GetPlayLists(db)

	fmt.Println(playlists)

	if err != nil {
		ctx.JSON(http.StatusNotFound, nil)
	}

	ctx.JSON(http.StatusOK, playlists)
}

func ChangePlaylistName(ctx *gin.Context, db *sql.DB, playlistID int, newName string) {
	err := crud.ChangePlaylistName(db, playlistID, newName)

	fmt.Println(err)

	if err != nil {
		ctx.JSON(http.StatusNotFound, err)
	}

	ctx.JSON(http.StatusOK, nil)
}

func CreatePlaylist(ctx *gin.Context, db *sql.DB, title string) {
	err := crud.CreatePlaylist(db, title)

	fmt.Println(err)

	if err != nil {
		ctx.JSON(http.StatusNotFound, err)
	}

	ctx.JSON(http.StatusOK, nil)
}

func RemovePlaylist(ctx *gin.Context, db *sql.DB, playlistId int) {
	err := crud.RemovePlaylist(db, playlistId)

	fmt.Println(err)

	if err != nil {
		ctx.JSON(http.StatusNotFound, err)
	}

	ctx.JSON(http.StatusOK, nil)
}

func RemoveSongFromPlaylist(ctx *gin.Context, db *sql.DB, playlistID int, songId string) {
	err := crud.RemoveSongFromPlaylist(db, playlistID, songId)

	fmt.Println(err)

	if err != nil {
		ctx.JSON(http.StatusNotFound, err)
	}

	ctx.JSON(http.StatusOK, nil)
}

func AddSongToPlaylist(ctx *gin.Context, db *sql.DB, playlistID int, songId string) {
	err := crud.AddSongToPlaylist(db, playlistID, songId)

	fmt.Println(err)

	if err != nil {
		ctx.JSON(http.StatusNotFound, err)
	}

	ctx.JSON(http.StatusOK, nil)
}

func RunServer(db *sql.DB) {

	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, PUT, POST, DELETE, OPTIONS, PATCH")
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

	router.GET("/api/searchYT", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.GET("/api/queue", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.GET("/api/currentlyPlaying", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.PATCH("/api/updatePlaylistName", func(ctx *gin.Context) {
		var body struct {
			PlaylistId int    `json:"playlist_id"`
			NewName    string `json:"title"`
		}
		if err := ctx.ShouldBindJSON(&body); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ChangePlaylistName(ctx, db, body.PlaylistId, body.NewName)
	})

	router.DELETE("/api/removeSongFromPlaylist", func(ctx *gin.Context) {
		var body struct {
			PlaylistId int    `json:"playlist_id"`
			SongId     string `json:"song_id"`
		}
		if err := ctx.ShouldBindJSON(&body); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		RemoveSongFromPlaylist(ctx, db, body.PlaylistId, body.SongId)
	})

	router.POST("/api/addSongToPlaylist", func(ctx *gin.Context) {
		var body struct {
			PlaylistId int    `json:"playlist_id"`
			SongId     string `json:"song_id"`
		}
		if err := ctx.ShouldBindJSON(&body); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		AddSongToPlaylist(ctx, db, body.PlaylistId, body.SongId)
	})

	router.POST("/api/createPlaylist", func(ctx *gin.Context) {
		var body struct {
			Title string `json:"title"`
		}
		if err := ctx.ShouldBindJSON(&body); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		CreatePlaylist(ctx, db, body.Title)
	})

	router.DELETE("/api/removePlaylist", func(ctx *gin.Context) {
		var body struct {
			PlaylistId int `json:"playlist_id"`
		}

		if err := ctx.ShouldBindJSON(&body); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		RemovePlaylist(ctx, db, body.PlaylistId)
	})

	router.Run("127.0.0.1:8080")
}
