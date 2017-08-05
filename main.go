package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func downloadFile(url string, destination string) (err error) {
	// Create the file
	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Writer the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		println(err)
		return err
	}

	return nil
}

func handleDownload(fileURL string) (string, bool) {
	arr := strings.Split(fileURL, "/")
	filename := arr[len(arr)-1]

	if !strings.HasSuffix(filename, ".pdf") {
		return fmt.Sprintf("invalid filename %s", filename), false
	}

	destinationPath := fmt.Sprintf("tmp/%s", filename)
	if downloadFile(fileURL, destinationPath) != nil {
		return fmt.Sprintf("error downloading %s", fileURL), false
	}

	// os.Getenv("API_KEY")
	publicURL := fmt.Sprintf("%s/download/%s", os.Getenv("BASE_URL"), filename)
	return string(publicURL), true
}

func storeHandler(c *gin.Context) {
	apiKey := os.Getenv("API_KEY")
	apiKeyParam := c.Query("api_key")
	if apiKey == apiKeyParam {
		link, success := handleDownload(c.Query("url"))
		if success {
			c.JSON(http.StatusOK, gin.H{"link": link})
		} else {
			c.JSON(http.StatusOK, gin.H{"error": link})
		}
	} else {
		c.String(http.StatusUnauthorized, fmt.Sprintf("invalid api key %s", apiKeyParam))
	}
}

func downloadHandler(c *gin.Context) {
	filepath := fmt.Sprintf("tmp/%s", c.Param("filename"))
	c.File(filepath)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	port := os.Getenv("PORT")

	if port == "" {
		log.Fatal("$PORT must be set")
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.LoadHTMLGlob("templates/*.tmpl.html")
	router.Static("/static", "static")

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl.html", nil)
	})

	router.GET("/download/:filename", downloadHandler)
	router.GET("/store", storeHandler)

	router.Run(":" + port)
}
