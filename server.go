package main

import (
	"database/sql"
	"discord_bot/crud"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

type Secret struct {
	ServerID            string
	ClientID            string
	random_state_string string
	ClientSecret        string
}

var secret Secret

var queueMoqup []Song = []Song{
	{Filename: "h4F-zhMz4PQ.mp3", Title: "asdf"},
	{Filename: "LMGAkdA41f8.mp3", Title: "test"},
	{Filename: "qrxv0JNVtgY.mp3", Title: "test2"},
}

var currentSongMockup Song = Song{}

func getOAuthConfig(r *http.Request) *oauth2.Config {

	bytes, err := os.ReadFile("secret.json") // replaces ioutil.ReadFile

	if err != nil {
		log.Fatal("Error reading file:", err)
	}

	if err := json.Unmarshal(bytes, &secret); err != nil {
		log.Fatal("Error decoding JSON:", err)
	}

	scheme := "http"
	host := r.Host
	return &oauth2.Config{
		ClientID:     secret.ClientID,
		ClientSecret: secret.ClientSecret,
		RedirectURL:  fmt.Sprintf("%s://%s/auth/callback", scheme, host),
		Scopes:       []string{"identify", "guilds"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://discord.com/api/oauth2/authorize",
			TokenURL: "https://discord.com/api/oauth2/token",
		},
	}
}

var YOUR_SERVER_ID = "684480426446028822" // your guild ID from DB tmp

func GetSongs(ctx *gin.Context, db *sql.DB) {
	songs, err := crud.GetSongs(db)

	if err != nil {
		ctx.JSON(http.StatusNotFound, nil)
	}

	ctx.JSON(http.StatusOK, songs)
}

func DownloadSelectedVideo(ctx *gin.Context, queryId string) {
	err := DownloadVideo(queryId)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to download video"})
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "downloaded"})
}

func GetVideoID(ctx *gin.Context, query string) {
	songs, err := GetVideoIDFromQuerry4(query)
	if err != nil {
		fmt.Println(err)
		ctx.JSON(http.StatusNotFound, gin.H{"error": "there was an error while trying to find"})
		return
	}

	var result []gin.H
	for _, song := range songs {
		result = append(result, gin.H{
			"title":    song.Title,
			"filename": song.Filename,
			"channel":  "tester",
			"duration": "4:30",
		})
	}

	ctx.JSON(http.StatusOK, result)
}

func GetPlaylists(ctx *gin.Context, db *sql.DB) {
	playlists, err := crud.GetPlayLists(db)

	// fmt.Println(playlists)

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
	id, err := crud.CreatePlaylist(db, title)

	fmt.Println(err)

	if err != nil {
		ctx.JSON(http.StatusNotFound, err)
	}

	ctx.JSON(http.StatusOK, gin.H{"playlist_id": id})
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
	url := getOAuthConfig(c.Request).AuthCodeURL("random_state_string")
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
	token, err := getOAuthConfig(c.Request).Exchange(c, code)
	// fmt.Println(token)
	// fmt.Println(code)
	fmt.Println(err)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token exchange failed"})
		return
	}

	// fetch user info from Discord
	client := getOAuthConfig(c.Request).Client(c, token)
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
	c.Redirect(http.StatusTemporaryRedirect, "/")
}

// middleware — protects all /api routes
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		// fmt.Println(userID)
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

func RestartServer(ctx *gin.Context) {
	process, err := os.FindProcess(os.Getegid())
	if err != nil {
		fmt.Println("There was an error while trying to kill the process")
	}

	process.Kill()
	ctx.JSON(http.StatusOK, gin.H{"message": "Restarting"})
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
	// api := router.Group("/api")
	{
		router.GET("/restart", func(ctx *gin.Context) {
			RestartServer(ctx)
		})

		router.GET("/songs", func(ctx *gin.Context) {
			GetSongs(ctx, db)
		})

		router.GET("/playlists", func(ctx *gin.Context) {
			GetPlaylists(ctx, db)
		})

		router.POST("/playThis", func(ctx *gin.Context) {
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

		router.GET("/searchYT", func(ctx *gin.Context) {
			query := ctx.Query("query")
			if query == "" {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "query is required"})
				return
			}
			// fmt.Println(query)
			GetVideoID(ctx, query)
		})

		router.GET("/downloadYT", func(ctx *gin.Context) {
			query := ctx.Query("query")
			if query == "" {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "query is required"})
				return
			}

			videoId := strings.Split(query, ".")[0]

			// fmt.Println(videoId)
			DownloadSelectedVideo(ctx, videoId)
		})

		router.GET("/queue", func(ctx *gin.Context) {
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

		router.POST("/queue/add", func(ctx *gin.Context) {
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

		router.POST("/queue/update", func(ctx *gin.Context) {
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

		router.GET("/currentlyPlaying", func(ctx *gin.Context) {
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

		router.POST("/nextSong", func(ctx *gin.Context) {
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

		router.PATCH("/updatePlaylistName", func(ctx *gin.Context) {
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

		router.DELETE("/removeSongFromPlaylist", func(ctx *gin.Context) {
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

		router.POST("/addSongToPlaylist", func(ctx *gin.Context) {
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

		router.POST("/createPlaylist", func(ctx *gin.Context) {
			var body struct {
				Title string `json:"title"`
			}
			if err := ctx.ShouldBindJSON(&body); err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			CreatePlaylist(ctx, db, body.Title)
		})

		router.DELETE("/removePlaylist", func(ctx *gin.Context) {
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

	// router.GET("/", func(ctx *gin.Context) {
	// 	ctx.HTML(http.StatusOK, "discord-music-bot.html", nil)
	// })

	router.Run(":6970")
}
