package main

import (
	"database/sql"
	"net/http"

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

	router.LoadHTMLGlob("templates/*")

	router.GET("/", func(ctx *gin.Context) {
		ctx.HTML(http.StatusOK, "discord-music-bot.html", nil)
	})

	router.Run(":6969")
}
