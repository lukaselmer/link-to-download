package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awsutil"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var db *sql.DB

func downloadFileFromURL(url string, destination string) (err error) {
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

func handleDownload(fileURL string) (response gin.H, err error) {
	arr := strings.Split(fileURL, "/")
	filename := arr[len(arr)-1]

	if !strings.HasSuffix(filename, ".pdf") || len(filename) < 5 {
		response = gin.H{"error": "invalid filename " + filename}
		return
	}

	id, err := createDBFile(fileURL, filename)
	if err != nil {
		response = gin.H{"error": fmt.Sprintf("error creating db file %s: %q", fileURL, err)}
		return
	}

	err = downloadFile(id, fileURL)
	if err != nil {
		response = gin.H{"error": fmt.Sprintf("error downloading %s: %q", fileURL, err)}
		return
	}

	uploadToS3(id)
	if err != nil {
		response = gin.H{"error": fmt.Sprintf("error uploading %s: %q", fileURL, err)}
		return
	}

	response = gin.H{
		"temporaryLink":  temporaryLink(id),
		"persistentLink": persistentLink(id)}
	return
}

func downloadFile(id int, fileURL string) error {
	destinationPath := fmt.Sprintf("tmp/%d.pdf", id)
	return downloadFileFromURL(fileURL, destinationPath)
}

func temporaryLink(id int) string {
	return fmt.Sprintf("%s/download/%d.pdf", os.Getenv("BASE_URL"), id)
}

func persistentLink(id int) string {
	return fmt.Sprintf("https://s3-%s.amazonaws.com/%s/files/%d.pdf",
		os.Getenv("AWS_REGION"), os.Getenv("AWS_BUCKET"), id)
}

func createDBFile(fileURL string, filename string) (int, error) {
	var id int
	err := db.QueryRow(
		"INSERT INTO files (origin_url, filename) VALUES ($1, $2) RETURNING id",
		fileURL,
		filename,
	).Scan(&id)

	if err != nil {
		return -1, fmt.Errorf("Error inserting file %s: %q", filename, err)
	}

	return id, nil
}

func uploadToS3(id int) error {
	creds := credentials.NewEnvCredentials()
	creds.Get()

	_, err := creds.Get()
	if err != nil {
		return fmt.Errorf("bad credentials: %s", err)
	}

	cfg := aws.NewConfig().WithRegion(os.Getenv("AWS_REGION")).WithCredentials(creds)
	svc := s3.New(session.New(), cfg)

	file, err := os.Open(fmt.Sprintf("tmp/%d.pdf", id))
	if err != nil {
		return fmt.Errorf("err opening file: %q", err)
	}
	defer file.Close()

	fileInfo, _ := file.Stat()
	var size = fileInfo.Size()

	buffer := make([]byte, size)
	file.Read(buffer)
	fileBytes := bytes.NewReader(buffer)
	fileType := http.DetectContentType(buffer)

	path := fmt.Sprintf("files/%d.pdf", id)
	params := &s3.PutObjectInput{
		Bucket:        aws.String(os.Getenv("AWS_BUCKET")),
		Key:           aws.String(path),
		Body:          fileBytes,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(fileType),
	}
	resp, err := svc.PutObject(params)
	if err != nil {
		return fmt.Errorf("bad response: %s", err)
	}
	fmt.Printf("response %s", awsutil.StringValue(resp))

	return nil
}

func storeURL(c *gin.Context, url string) {
	apiKey := os.Getenv("API_KEY")
	apiKeyParam := c.Query("api_key")
	if apiKey == apiKeyParam {
		response, err := handleDownload(url)
		if err == nil {
			c.JSON(http.StatusOK, response)
		} else {
			c.JSON(http.StatusUnprocessableEntity, response)
		}
	} else {
		c.String(http.StatusUnauthorized, fmt.Sprintf("invalid api key %s", apiKeyParam))
	}
}

func storeHandler(c *gin.Context) {
	storeURL(c, c.Query("url"))
}

func downloadHandler(c *gin.Context) {
	filepath := fmt.Sprintf("tmp/%s", c.Param("filename"))
	c.File(filepath)
}

func extractURL(message string) string {
	re := regexp.MustCompile("(https:\\/\\/\\S+\\.pdf)")
	match := re.FindStringSubmatch(message)
	if len(match) >= 2 {
		return match[1]
	}
	return ""
}

func storeFromTextHandler(c *gin.Context) {
	message := c.PostForm("message")
	url := extractURL(message)
	if len(url) > 0 {
		storeURL(c, url)
	}
}

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
