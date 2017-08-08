package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var db *sql.DB

func main() {
	initEnvVariables()
	initDB()
	startServer()
}

func initEnvVariables() {
	err := godotenv.Load()
	if err != nil {
		log.Print("Config from .env file was not loaded")
	}

	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("$PORT must be set")
	}
}

func initDB() {
	var err error

	db, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("Error opening database: %q", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("Unable to ping the database: %q", err)
	}

	statement := "CREATE TABLE IF NOT EXISTS files (" +
		"id serial PRIMARY KEY," +
		"origin_url text NOT NULL," +
		"filename text NOT NULL," +
		"created_at timestamp NOT NULL DEFAULT now()" +
		")"
	if _, err := db.Exec(statement); err != nil {
		log.Fatalf("Error creating database table: %q", err)
	}
}

func startServer() {
	router := gin.New()
	router.Use(gin.Logger())
	router.LoadHTMLGlob("templates/*.tmpl.html")
	router.Static("/static", "static")

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl.html", nil)
	})

	router.GET("/download/:filename", downloadHandler)
	router.GET("/store", storeHandler)
	router.POST("/store-from-text", storeFromTextHandler)

	router.Run(":" + os.Getenv("PORT"))
}

func downloadHandler(c *gin.Context) {
	filepath := fmt.Sprintf("tmp/%s", c.Param("filename"))
	c.File(filepath)
}

func storeHandler(c *gin.Context) {
	storeURL(c, c.Query("url"))
}

func storeFromTextHandler(c *gin.Context) {
	message := c.PostForm("message")
	url := extractURL(message)
	if len(url) > 0 {
		storeURL(c, url)
	}
}

func extractURL(message string) string {
	re := regexp.MustCompile("(https:\\/\\/\\S+\\.pdf)")
	match := re.FindStringSubmatch(message)
	if len(match) >= 2 {
		return match[1]
	}
	return ""
}
