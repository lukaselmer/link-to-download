package main

import (
	"bytes"
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

func handleDownload(fileURL string) (gin.H, bool) {
	arr := strings.Split(fileURL, "/")
	filename := arr[len(arr)-1]

	// the filename validation is very basic and possibly enables a security issues
	if !strings.HasSuffix(filename, ".pdf") {
		return gin.H{"error": "invalid filename " + filename}, false
	}

	destinationPath := fmt.Sprintf("tmp/%s", filename)
	if downloadFile(fileURL, destinationPath) != nil {
		return gin.H{"error": "error downloading " + fileURL}, false
	}

	uploadToS3(filename)

	publicURL := fmt.Sprintf("%s/download/%s", os.Getenv("BASE_URL"), filename)

	publicAwsURL := fmt.Sprintf("https://s3-%s.amazonaws.com/%s/files/%s",
		os.Getenv("AWS_REGION"), os.Getenv("AWS_BUCKET"), filename)

	return gin.H{"temporaryLink": publicURL, "persistentLink": publicAwsURL}, true
}

func uploadToS3(filename string) {
	creds := credentials.NewEnvCredentials()
	creds.Get()

	_, err := creds.Get()
	if err != nil {
		fmt.Printf("bad credentials: %s", err)
	}

	cfg := aws.NewConfig().WithRegion(os.Getenv("AWS_REGION")).WithCredentials(creds)
	svc := s3.New(session.New(), cfg)

	file, err := os.Open("tmp/" + filename)
	if err != nil {
		fmt.Printf("err opening file: %s", err)
	}
	defer file.Close()

	fileInfo, _ := file.Stat()
	var size = fileInfo.Size()

	buffer := make([]byte, size)
	file.Read(buffer)
	fileBytes := bytes.NewReader(buffer)
	fileType := http.DetectContentType(buffer)

	path := "files/" + filename
	params := &s3.PutObjectInput{
		Bucket:        aws.String(os.Getenv("AWS_BUCKET")),
		Key:           aws.String(path),
		Body:          fileBytes,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(fileType),
	}
	resp, err := svc.PutObject(params)
	if err != nil {
		fmt.Printf("bad response: %s", err)
	}
	fmt.Printf("response %s", awsutil.StringValue(resp))
}

func storeURL(c *gin.Context, url string) {
	apiKey := os.Getenv("API_KEY")
	apiKeyParam := c.Query("api_key")
	if apiKey == apiKeyParam {
		response, success := handleDownload(url)
		if success {
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
	err := godotenv.Load()
	if err != nil {
		log.Print("Config from .env file was not loaded")
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
	router.POST("/store-from-text", storeFromTextHandler)

	router.Run(":" + port)
}
