package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// Config ładowany z pliku JSON
type ConfigSrv struct {
	APIBaseURL string `json:"api_base_url"`
}

func loadConfig(path string) (*ConfigSrv, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("nie można odczytać pliku konfiguracyjnego: %w", err)
	}
	var cfg ConfigSrv
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("błąd parsowania JSON: %w", err)
	}
	return &cfg, nil
}

// Pomocnicze funkcje do proxowania requestów

func proxyGET(ctx *gin.Context, url string) {
	resp, err := http.Get(url)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": "błąd połączenia z API: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	ctx.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

func proxyPOST(ctx *gin.Context, url string, payload any) {
	proxyWithMethod(ctx, http.MethodPost, url, payload)
}

func proxyPATCH(ctx *gin.Context, url string, payload any) {
	proxyWithMethod(ctx, http.MethodPatch, url, payload)
}

func proxyDELETE(ctx *gin.Context, url string, payload any) {
	proxyWithMethod(ctx, http.MethodDelete, url, payload)
}

func proxyWithMethod(ctx *gin.Context, method, url string, payload any) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "błąd serializacji: " + err.Error()})
		return
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "błąd tworzenia requestu: " + err.Error()})
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": "błąd połączenia z API: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	ctx.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

func RunFrontServer() {
	cfg, err := loadConfig("config.json")
	if err != nil {
		panic(err)
	}
	api := cfg.APIBaseURL // np. "http://localhost:8080"

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

	store := cookie.NewStore([]byte("random_state_string"))
	store.Options(sessions.Options{
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		MaxAge:   60 * 60 * 24,
	})
	router.Use(sessions.Sessions("discord_session", store))

	// Auth routes — bez middleware
	router.GET("/auth/login", handleLogin)
	router.GET("/auth/callback", handleCallback)
	router.GET("/auth/logout", handleLogout)

	// Chronione endpointy — proxy do zewnętrznego API
	apiGroup := router.Group("/api")
	apiGroup.Use(authMiddleware())
	{
		// GET /api/songs
		apiGroup.GET("/songs", func(ctx *gin.Context) {
			proxyGET(ctx, api+"/songs")
		})

		// GET /api/playlists
		apiGroup.GET("/playlists", func(ctx *gin.Context) {
			proxyGET(ctx, api+"/playlists")
		})

		// POST /api/playThis
		apiGroup.POST("/playThis", func(ctx *gin.Context) {
			var body struct {
				SongId   string `json:"SongId"`
				SongName string `json:"SongName"`
			}
			if err := ctx.ShouldBindJSON(&body); err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			proxyPOST(ctx, api+"/playThis", body)
		})

		// GET /api/searchYT?query=...
		apiGroup.GET("/searchYT", func(ctx *gin.Context) {
			query := ctx.Query("query")
			if query == "" {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "query is required"})
				return
			}
			proxyGET(ctx, api+"/searchYT?query="+query)
		})

		// GET /api/downloadYT?query=...
		apiGroup.GET("/downloadYT", func(ctx *gin.Context) {
			query := ctx.Query("query")
			if query == "" {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "query is required"})
				return
			}
			videoId := strings.Split(query, ".")[0]
			proxyGET(ctx, api+"/downloadYT?query="+videoId)
		})

		// GET /api/queue
		apiGroup.GET("/queue", func(ctx *gin.Context) {
			proxyGET(ctx, api+"/queue")
		})

		// POST /api/queue/add
		apiGroup.POST("/queue/add", func(ctx *gin.Context) {
			var body struct {
				SongId   string `json:"SongId"`
				SongName string `json:"SongName"`
			}
			if err := ctx.ShouldBindJSON(&body); err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			proxyPOST(ctx, api+"/queue/add", body)
		})

		// POST /api/queue/update
		apiGroup.POST("/queue/update", func(ctx *gin.Context) {
			var items []struct {
				Id    string `json:"id"`
				Title string `json:"title"`
			}
			if err := ctx.ShouldBindJSON(&items); err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			proxyPOST(ctx, api+"/queue/update", items)
		})

		// GET /api/currentlyPlaying
		apiGroup.GET("/currentlyPlaying", func(ctx *gin.Context) {
			proxyGET(ctx, api+"/currentlyPlaying")
		})

		// POST /api/nextSong
		apiGroup.POST("/nextSong", func(ctx *gin.Context) {
			var body struct {
				SongId   string `json:"SongId"`
				SongName string `json:"SongName"`
			}
			if err := ctx.ShouldBindJSON(&body); err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			proxyPOST(ctx, api+"/nextSong", body)
		})

		// PATCH /api/updatePlaylistName
		apiGroup.PATCH("/updatePlaylistName", func(ctx *gin.Context) {
			var body struct {
				PlaylistId int    `json:"playlist_id"`
				NewName    string `json:"title"`
			}
			if err := ctx.ShouldBindJSON(&body); err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			proxyPATCH(ctx, api+"/updatePlaylistName", body)
		})

		// DELETE /api/removeSongFromPlaylist
		apiGroup.DELETE("/removeSongFromPlaylist", func(ctx *gin.Context) {
			var body struct {
				PlaylistId int    `json:"playlist_id"`
				SongId     string `json:"song_id"`
			}
			if err := ctx.ShouldBindJSON(&body); err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			proxyDELETE(ctx, api+"/removeSongFromPlaylist", body)
		})

		// POST /api/addSongToPlaylist
		apiGroup.POST("/addSongToPlaylist", func(ctx *gin.Context) {
			var body struct {
				PlaylistId int    `json:"playlist_id"`
				SongId     string `json:"song_id"`
			}
			if err := ctx.ShouldBindJSON(&body); err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			proxyPOST(ctx, api+"/addSongToPlaylist", body)
		})

		// POST /api/createPlaylist
		apiGroup.POST("/createPlaylist", func(ctx *gin.Context) {
			var body struct {
				Title string `json:"title"`
			}
			if err := ctx.ShouldBindJSON(&body); err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			proxyPOST(ctx, api+"/createPlaylist", body)
		})

		// DELETE /api/removePlaylist
		apiGroup.DELETE("/removePlaylist", func(ctx *gin.Context) {
			var body struct {
				PlaylistId int `json:"playlist_id"`
			}
			if err := ctx.ShouldBindJSON(&body); err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			proxyDELETE(ctx, api+"/removePlaylist", body)
		})
	}

	router.LoadHTMLGlob("templates/*")
	router.GET("/", func(ctx *gin.Context) {
		ctx.HTML(http.StatusOK, "discord-music-bot.html", nil)
	})

	router.Run(":6969")
}
