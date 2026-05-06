package main

import (
	"database/sql"
	"discord_bot/crud"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

var discordOAuth = &oauth2.Config{
	ClientID:     os.Getenv("DISCORD_CLIENT_ID"),
	ClientSecret: os.Getenv("DISCORD_CLIENT_SECRET"),
	RedirectURL:  "http://localhost:8080/auth/callback",
	Scopes:       []string{"identify", "guilds"},
	Endpoint: oauth2.Endpoint{
		AuthURL:  "https://discord.com/api/oauth2/authorize",
		TokenURL: "https://discord.com/api/oauth2/token",
	},
}

const YOUR_SERVER_ID = "123456789012345678" // your guild ID from DB tmp

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

func handleLogin(c *gin.Context) {
	url := discordOAuth.AuthCodeURL("random_state_string")
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func handleLogout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// Discord redirects back here with a code
func handleCallback(c *gin.Context) {
	code := c.Query("code")
	token, err := discordOAuth.Exchange(c, code)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token exchange failed"})
		return
	}

	// fetch user info from Discord
	client := discordOAuth.Client(c, token)
	resp, err := client.Get("https://discord.com/api/users/@me")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}
	defer resp.Body.Close()

	var discordUser struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	json.NewDecoder(resp.Body).Decode(&discordUser)

	// check if user is in your server
	guildsResp, err := client.Get("https://discord.com/api/users/@me/guilds")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get guilds"})
		return
	}
	defer guildsResp.Body.Close()

	var guilds []struct {
		ID string `json:"id"`
	}
	json.NewDecoder(guildsResp.Body).Decode(&guilds)

	// check if your server ID is in their guild list
	isMember := false
	for _, g := range guilds {
		if g.ID == YOUR_SERVER_ID {
			isMember = true
			break
		}
	}

	if !isMember {
		c.JSON(http.StatusForbidden, gin.H{"error": "you are not a member of this server"})
		return
	}

	// save to session cookie
	session := sessions.Default(c)
	session.Set("user_id", discordUser.ID)
	session.Set("username", discordUser.Username)
	session.Save()

	// redirect to frontend
	c.Redirect(http.StatusTemporaryRedirect, "http://localhost:3000")
}

// middleware — protects all /api routes
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		if userID == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			c.Abort()
			return
		}
		// make user_id available to handlers
		c.Set("user_id", userID)
		c.Next()
	}
}

func RunServer(db *sql.DB) {

	router := gin.Default()
	// router.Use(func(c *gin.Context) {
	// 	c.Header("Access-Control-Allow-Origin", "*")
	// 	c.Header("Access-Control-Allow-Methods", "GET, PUT, POST, DELETE, OPTIONS, PATCH")
	// 	c.Header("Access-Control-Allow-Headers", "Content-Type")
	// 	if c.Request.Method == "OPTIONS" {
	// 		c.AbortWithStatus(http.StatusNoContent)
	// 		return
	// 	}
	// 	c.Next()
	// })

	// router.Static("/static", "./static")

	// cookie session store
	store := cookie.NewStore([]byte(os.Getenv("SESSION_SECRET")))
	store.Options(sessions.Options{
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

		api.GET("/searchYT", func(ctx *gin.Context) {
			ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		api.GET("/queue", func(ctx *gin.Context) {
			fmt.Println(players[""].Queue)
			ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		api.GET("/currentlyPlaying", func(ctx *gin.Context) {
			ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
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

	router.Run("localhost:8080")
}
