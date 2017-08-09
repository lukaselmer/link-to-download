package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awsutil"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

// storeURL downloads a file from a given url.
// It also uploads the file to S3 and stores the metadata in the DB.
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

func downloadFile(id int, fileURL string) error {
	destinationPath := fmt.Sprintf("tmp/%d.pdf", id)
	return downloadFileFromURL(fileURL, destinationPath)
}

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
		return err
	}

	return nil
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

	path := s3RelativePath(id)
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

func s3RelativePath(id int) string {
	// To avoid one url leak to compromise all files, it would be wise
	// to hash the id and the api key to generate a unique path
	return fmt.Sprintf("files/%s/%d.pdf", os.Getenv("API_KEY"), id)
}

func temporaryLink(id int) string {
	return fmt.Sprintf("%s/download/%d.pdf", os.Getenv("BASE_URL"), id)
}

func persistentLink(id int) string {
	return fmt.Sprintf("https://s3-%s.amazonaws.com/%s/%s",
		os.Getenv("AWS_REGION"), os.Getenv("AWS_BUCKET"), s3RelativePath(id))
}
