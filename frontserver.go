package main

import (
	"database/sql"
	"discord_bot/crud"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func RunFrontServer(db *sql.DB) {
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

	router.SetTrustedProxies([]string{"127.0.0.1"})

	// router.Static("/static", "./static")

	// cookie session store
	store := cookie.NewStore([]byte("random_state_string"))
	store.Options(sessions.Options{
		Path:     "/",
		HttpOnly: true,
		Secure:   false,        // set true in production with HTTPS
		MaxAge:   60 * 60 * 24, // 1 day
	})

	router.Use(sessions.Sessions("discord_session", store))

	// auth routes — no middleware
	router.GET("/auth/login", handleLogin)
	router.GET("/auth/callback", handleCallback)
	router.GET("/auth/logout", handleLogout)

	// protected API routes
	api := router.Group("/api")
	api.Use(authMiddleware())
	{
		api.GET("/songs", func(ctx *gin.Context) {
			GetSongs(ctx, db)
		})

		api.GET("/playlists", func(ctx *gin.Context) {
			GetPlaylists(ctx, db)
		})

		api.POST("/playThis", func(ctx *gin.Context) {
			var body struct {
				SongId   string `json:"SongId"`
				SongName string `json:"SongName"`
			}

			if err := ctx.ShouldBindJSON(&body); err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			currentSongMockup = Song{Title: body.SongName, Filename: body.SongId}

			if players[YOUR_SERVER_ID] != nil {
				players[YOUR_SERVER_ID].PlayMusicFromWeb(currentSongMockup)
				crud.InsertSongIntoDatabase(currentSongMockup.Filename, currentSongMockup.Title, YOUR_SERVER_ID, db)
				crud.UpdateSongsPlayCount(currentSongMockup.Filename, YOUR_SERVER_ID, db)
			} else {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "there is no active player"})
				return
			}

			ctx.JSON(http.StatusOK, gin.H{"message": "ok"})
		})

		api.GET("/searchYT", func(ctx *gin.Context) {
			query := ctx.Query("query")
			if query == "" {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "query is required"})
				return
			}
			// fmt.Println(query)
			GetVideoID(ctx, query)
		})

		api.GET("/downloadYT", func(ctx *gin.Context) {
			query := ctx.Query("query")
			if query == "" {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "query is required"})
				return
			}

			videoId := strings.Split(query, ".")[0]

			// fmt.Println(videoId)
			DownloadSelectedVideo(ctx, videoId)
		})

		api.GET("/queue", func(ctx *gin.Context) {
			type QueueItem struct {
				Id     string `json:"id"`
				Title  string `json:"title"`
				Server string `json:"server"`
			}
			if players[YOUR_SERVER_ID] == nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "there is no active player"})
				return
			}
			queue := players[YOUR_SERVER_ID].Queue
			// queue := queueMoqup
			items := make([]QueueItem, len(queue))
			for i, song := range queue {
				items[i] = QueueItem{
					Id:     strings.TrimSuffix(song.Filename, ".mp3"),
					Title:  song.Title,
					Server: YOUR_SERVER_ID,
				}
			}

			ctx.JSON(http.StatusOK, items)
		})

		api.POST("/queue/add", func(ctx *gin.Context) {
			var body struct {
				SongId   string `json:"SongId"`
				SongName string `json:"SongName"`
			}

			if err := ctx.ShouldBindJSON(&body); err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			// fmt.Println(players[YOUR_SERVER_ID].Queue)

			if players[YOUR_SERVER_ID] == nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "there is no active player"})
				return
			}

			players[YOUR_SERVER_ID].Queue = append(players[YOUR_SERVER_ID].Queue, Song{Filename: body.SongId, Title: body.SongName})
			// queueMoqup = append(queueMoqup, Song{Filename: body.SongId, Title: body.SongName})
		})

		api.POST("/queue/update", func(ctx *gin.Context) {
			type QueueItem struct {
				Id    string `json:"id"`
				Title string `json:"title"`
			}

			var items []QueueItem
			if err := ctx.ShouldBindJSON(&items); err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			// rebuild the queue from the reordered items
			newQueue := make([]Song, len(items))
			for i, item := range items {
				newQueue[i] = Song{
					Filename: item.Id + ".mp3", // re-append the suffix
					Title:    item.Title,
				}
			}

			if players[YOUR_SERVER_ID] == nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "there is no active player"})
				return
			}

			players[YOUR_SERVER_ID].Queue = newQueue
			// queueMoqup = newQueue
			ctx.JSON(http.StatusOK, gin.H{"updated": len(newQueue)})
		})

		api.GET("/currentlyPlaying", func(ctx *gin.Context) {
			type NowPlayingItem struct {
				Id     string `json:"id"`
				Title  string `json:"title"`
				Server string `json:"server"`
			}

			if players[YOUR_SERVER_ID] == nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "there is no active player"})
				return
			}

			currentSong := players[YOUR_SERVER_ID].CurrentSong

			fmt.Println(currentSong)
			item := NowPlayingItem{
				Id:     strings.TrimSuffix(currentSong.Filename, ".mp3"),
				Title:  currentSong.Title,
				Server: YOUR_SERVER_ID,
			}
			// item := NowPlayingItem{
			// 	Id:     strings.TrimSuffix(currentSongMockup.Filename, ".mp3"),
			// 	Title:  currentSongMockup.Title,
			// 	Server: YOUR_SERVER_ID,
			// }
			// fmt.Println(currentSongMockup)
			ctx.JSON(http.StatusOK, gin.H{"CurrentSong": item})
		})

		api.POST("/nextSong", func(ctx *gin.Context) {
			var body struct {
				SongId   string `json:"SongId"`
				SongName string `json:"SongName"`
			}

			if err := ctx.ShouldBindJSON(&body); err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			nextSong := Song{Title: body.SongName, Filename: body.SongId}

			players[YOUR_SERVER_ID].SkipMusicFromWeb(nextSong)
			ctx.JSON(http.StatusOK, gin.H{"message": "skipping"})
		})

		api.PATCH("/updatePlaylistName", func(ctx *gin.Context) {
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

		api.DELETE("/removeSongFromPlaylist", func(ctx *gin.Context) {
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

		api.POST("/addSongToPlaylist", func(ctx *gin.Context) {
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

		api.POST("/createPlaylist", func(ctx *gin.Context) {
			var body struct {
				Title string `json:"title"`
			}
			if err := ctx.ShouldBindJSON(&body); err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			CreatePlaylist(ctx, db, body.Title)
		})

		api.DELETE("/removePlaylist", func(ctx *gin.Context) {
			var body struct {
				PlaylistId int `json:"playlist_id"`
			}

			if err := ctx.ShouldBindJSON(&body); err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			RemovePlaylist(ctx, db, body.PlaylistId)
		})
	}

	router.LoadHTMLGlob("templates/*")

	router.GET("/", func(ctx *gin.Context) {
		ctx.HTML(http.StatusOK, "discord-music-bot.html", nil)
	})

	router.Run(":6969")
}
