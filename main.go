package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var (
	objKey     string = "myobjpath/hello.txt"
	uploadFrom string = "hello.txt"
	downloadTo string = "hello-downloaded.txt"
)

var (
	s3Endpoint = os.Getenv("S3_ENDPOINT")
	s3Region   = os.Getenv("S3_REGION")
	s3Bucket   = os.Getenv("S3_BUCKET")
	accessKey  = os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey  = os.Getenv("AWS_SECRET_KEY")
)

type S3PresignGetObjectAPI interface {
	PresignGetObject(
		ctx context.Context,
		params *s3.GetObjectInput,
		optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

func GetPresignedURL(c context.Context, api S3PresignGetObjectAPI, input *s3.GetObjectInput) (*v4.PresignedHTTPRequest, error) {
	return api.PresignGetObject(c, input, s3.WithPresignExpires(15*time.Minute))
}

func exitErrorf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

func main() {
	creds := credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			PartitionID:       "aws",
			URL:               s3Endpoint,
			SigningRegion:     s3Region,
			HostnameImmutable: true,
		}, nil
	})
	config := aws.Config{
		Credentials:                 creds,
		EndpointResolverWithOptions: customResolver,
		Region:                      s3Region,
		RetryMaxAttempts:            3,
	}
	client := s3.NewFromConfig(config)

	fromFile, err := os.Open(uploadFrom)
	if err != nil {
		exitErrorf("Unable to open file %q, %v", uploadFrom, err)
	}
	defer fromFile.Close()

	uploader := manager.NewUploader(client)
	// s3 will overwrite the existing file when you upload the file with same name
	_, err = uploader.Upload(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(s3Bucket),
		Key:         aws.String(objKey),
		ACL:         types.ObjectCannedACLPublicRead,
		ContentType: aws.String("text/plain"),
		Body:        fromFile,
		Expires:     aws.Time(time.Now().Local().Add(3 * time.Hour)),
	})
	if err != nil {
		// Print the error and exit.
		exitErrorf("Unable to upload %q to %q, %v", uploadFrom, s3Bucket, err)
	}

	fmt.Printf("Successfully uploaded %q to %q\n", uploadFrom, s3Bucket)

	toFile, err := os.Create(downloadTo)
	if err != nil {
		exitErrorf("Unable to open file %q, %v", downloadTo, err)
	}
	downloader := manager.NewDownloader(client)
	numBytes, err := downloader.Download(context.Background(), toFile,
		&s3.GetObjectInput{
			Bucket: aws.String(s3Bucket),
			Key:    aws.String(objKey),
		})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			exitErrorf("object with key %s does not exist in bucket %s", objKey, s3Bucket)
		}
		var nsb *types.NoSuchBucket
		if errors.As(err, &nsb) {
			exitErrorf("bucket %s does not exist", s3Bucket)
		}
		exitErrorf("Unable to download item %q, %v", objKey, err)
	}

	fmt.Println("Downloaded", toFile.Name(), numBytes, "bytes")

	psClient := s3.NewPresignClient(client)
	resp, err := GetPresignedURL(context.Background(), psClient, &s3.GetObjectInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(objKey),
	})
	if err != nil {
		exitErrorf("Got an error retrieving pre-signed object: %v", err)
	}
	fmt.Printf("Get presigned URL %q\n", resp.URL)

	objs, err := client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{Bucket: aws.String(s3Bucket)})
	if err != nil {
		exitErrorf("Unable to list items in bucket %q, %v", s3Bucket, err)
	}

	for _, item := range objs.Contents {
		fmt.Println("Name:         ", *item.Key)
		fmt.Println("Last modified:", *item.LastModified)
		fmt.Println("Size (byte):         ", item.Size)
		fmt.Println("")
	}

	_, err = client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(objKey),
		Body:   toFile,
	})
	if err != nil {
		exitErrorf("Could not put %s in bucket %s, %v", objKey, s3Bucket, err)
	}
	fmt.Printf("Put %q in bucket %q successfully\n", objKey, s3Bucket)

	_, err = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(objKey),
	})
	if err != nil {
		exitErrorf("Could not delete %s in bucket %s, %v", objKey, s3Bucket, err)
	}
	fmt.Printf("%q in bucket %q is deleted\n", objKey, s3Bucket)
}
