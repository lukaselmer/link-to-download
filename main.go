package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

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
	router.SetFuncMap(template.FuncMap{
		"persistentLink": persistentLink,
	})
	router.LoadHTMLGlob("templates/*.tmpl.html")

	router.Static("/static", "static")
	router.GET("/", handleIndex)
	router.GET("/download/:filename", downloadHandler)
	router.GET("/store", storeHandler)
	router.POST("/store-from-text", storeFromTextHandler)

	router.Run(":" + os.Getenv("PORT"))
}

func handleIndex(c *gin.Context) {
	if os.Getenv("API_KEY") == c.Query("api_key") {
		rows, err := db.Query("SELECT * FROM files ORDER BY id DESC")
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error reading files from db: %q", err))
			return
		}
		defer rows.Close()

		type DownloadedFile struct {
			ID        int
			OriginURL string
			Filename  string
			CreatedAt time.Time
		}
		var downloadedFiles []DownloadedFile

		for rows.Next() {
			var downloadedFile DownloadedFile
			if err := rows.Scan(
				&downloadedFile.ID,
				&downloadedFile.OriginURL,
				&downloadedFile.Filename,
				&downloadedFile.CreatedAt); err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintf("Error scanning downloadedFile: %q", err))
				return
			}
			downloadedFiles = append(downloadedFiles, downloadedFile)
		}
		c.HTML(http.StatusOK, "index.tmpl.html", downloadedFiles)
	} else {
		c.HTML(http.StatusOK, "index.tmpl.html", nil)
	}
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
