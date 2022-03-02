package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

var (
	uploader *s3manager.Uploader

	s3Endpoint = os.Getenv("S3_ENDPOINT")
	s3Region   = os.Getenv("S3_REGION")
	s3Bucket   = os.Getenv("S3_BUCKET")
	accessKey  = os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey  = os.Getenv("AWS_SECRET_KEY")
)

func init() {
	creds := credentials.NewStaticCredentials(accessKey, secretKey, "")

	config := &aws.Config{
		Credentials:      creds,
		Endpoint:         aws.String(s3Endpoint),
		Region:           aws.String(s3Region),
		DisableSSL:       aws.Bool(true),
		S3ForcePathStyle: aws.Bool(true),
		MaxRetries:       aws.Int(3),
	}

	sess := session.Must(session.NewSession(config))
	uploader = s3manager.NewUploader(sess)
}

func main() {
	r := gin.Default()
	r.POST("/myfile", saveFileHandler)
	r.Run(":8088")
}

func saveFileHandler(c *gin.Context) {
	fileHeader, err := c.FormFile("file")

	// The file cannot be received.
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"message": "No file is received",
		})
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"message": "fail to open file",
		})
		return
	}

	extension := filepath.Ext(fileHeader.Filename)
	newFileName := uuid.New().String() + extension
	if err := putFileToS3(c.Request.Context(), s3Bucket, newFileName, f); err != nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"message": "fail to upload to s3",
		})
		return
	}

	c.JSON(http.http.StatusCreated, gin.H{
		"message": fmt.Sprintf("file uploaded: %s", newFileName),
	})
}

func putFileToS3(ctx context.Context, bucket, fileName string, f io.Reader) error {
	_, err := uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(fileName),
		Body:   f,
	})
	if err != nil {
		return err
	}
	return nil
}
