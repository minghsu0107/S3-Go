package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

var (
	newBucket  string = "mynewbucket"
	objKey     string = "mysubpath/hello.txt"
	uploadFrom string = "hello.txt"
	downloadTo string = "hello-downloaded.txt"
)

var (
	s3Endpoint = os.Getenv("S3_ENDPOINT")
	s3Region   = os.Getenv("S3_REGION")
	accessKey  = os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey  = os.Getenv("AWS_SECRET_KEY")
)

func main() {
	creds := credentials.NewStaticCredentials(accessKey, secretKey, "")

	config := &aws.Config{
		Credentials:      creds,
		Endpoint:         aws.String(s3Endpoint),
		Region:           aws.String(s3Region),
		DisableSSL:       aws.Bool(true),
		S3ForcePathStyle: aws.Bool(true),
		MaxRetries:       aws.Int(3),
	}
	session, err := session.NewSession(config)
	if err != nil {
		fmt.Println("failed to create session", err)
		return
	}
	client := s3.New(session)

	result, err := client.ListBuckets(nil)
	if err != nil {
		fmt.Printf("Unable to list buckets, %v", err)
		return
	}
	fmt.Println("Buckets:")
	for _, b := range result.Buckets {
		fmt.Printf("* %s created on %s\n",
			aws.StringValue(b.Name), aws.TimeValue(b.CreationDate))
	}

	if len(result.Buckets) > 0 {
		i := 0
		err = client.ListObjectsPages(&s3.ListObjectsInput{
			Bucket: result.Buckets[0].Name,
		}, func(p *s3.ListObjectsOutput, last bool) (shouldContinue bool) {
			fmt.Println("Page,", i)
			i++

			for _, obj := range p.Contents {
				fmt.Println("Object:", *obj.Key)
			}
			return true
		})
		if err != nil {
			fmt.Println("failed to list objects", err)
			return
		}
	}

	_, err = client.CreateBucketWithContext(context.Background(), &s3.CreateBucketInput{
		Bucket: aws.String(newBucket),
	})
	if err != nil {
		exitErrorf("Unable to create bucket %q, %v", newBucket, err)
	}

	// Wait until bucket is created before finishing
	fmt.Printf("Waiting for bucket %q to be created...\n", newBucket)

	err = client.WaitUntilBucketExists(&s3.HeadBucketInput{
		Bucket: aws.String(newBucket),
	})

	if err != nil {
		exitErrorf("Error occurred while waiting for bucket to be created, %v", newBucket)
	}

	fmt.Printf("Bucket %q successfully created\n", newBucket)

	fromFile, err := os.Open(uploadFrom)
	if err != nil {
		exitErrorf("Unable to open file %q, %v", uploadFrom, err)
	}
	defer fromFile.Close()

	uploader := s3manager.NewUploader(session)
	// s3 will overwrite the existing file when you upload the file with same name
	_, err = uploader.UploadWithContext(context.Background(), &s3manager.UploadInput{
		Bucket:      aws.String(newBucket),
		Key:         aws.String(objKey),
		ACL:         aws.String("public-read"),
		ContentType: aws.String("text/plain"),
		Body:        fromFile,
		Expires:     aws.Time(time.Now().Local().Add(3 * time.Hour)),
	})
	if err != nil {
		// Print the error and exit.
		exitErrorf("Unable to upload %q to %q, %v", uploadFrom, newBucket, err)
	}

	fmt.Printf("Successfully uploaded %q to %q\n", uploadFrom, newBucket)

	toFile, err := os.Create(downloadTo)
	if err != nil {
		exitErrorf("Unable to open file %q, %v", downloadTo, err)
	}
	downloader := s3manager.NewDownloader(session)
	numBytes, err := downloader.DownloadWithContext(context.Background(), toFile,
		&s3.GetObjectInput{
			Bucket: aws.String(newBucket),
			Key:    aws.String(objKey),
		})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchBucket:
				exitErrorf("bucket %s does not exist", os.Args[1])
			case s3.ErrCodeNoSuchKey:
				exitErrorf("object with key %s does not exist in bucket %s", os.Args[2], os.Args[1])
			}
		}
		exitErrorf("Unable to download item %q, %v", objKey, err)
	}

	fmt.Println("Downloaded", toFile.Name(), numBytes, "bytes")

	req, _ := client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(newBucket),
		Key:    aws.String(objKey),
	})
	urlStr, err := req.Presign(15 * time.Minute)

	if err != nil {
		fmt.Println("Failed to sign request", err)
	}

	fmt.Printf("Get presigned URL %q\n", urlStr)

	resp, err := client.ListObjectsV2(&s3.ListObjectsV2Input{Bucket: aws.String(newBucket)})
	if err != nil {
		exitErrorf("Unable to list items in bucket %q, %v", newBucket, err)
	}

	for _, item := range resp.Contents {
		fmt.Println("Name:         ", *item.Key)
		fmt.Println("Last modified:", *item.LastModified)
		fmt.Println("Size (byte):         ", *item.Size)
		fmt.Println("")
	}

	_, err = client.DeleteObjectWithContext(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(newBucket),
		Key:    aws.String(objKey),
	})
	if err != nil {
		exitErrorf("Could not delete %s in bucket %s, %v", objKey, newBucket, err)
	}
	fmt.Printf("%q in bucket %q is deleted\n", objKey, newBucket)

	_, err = client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(newBucket),
		Key:    aws.String(objKey),
		Body:   toFile,
	})
	if err != nil {
		exitErrorf("Could not put %s in bucket %s, %v", objKey, newBucket, err)
	}
	fmt.Printf("Put %q in bucket %q successfully\n", objKey, newBucket)

	iter := s3manager.NewDeleteListIterator(client, &s3.ListObjectsInput{
		Bucket: aws.String(newBucket),
	})

	if err := s3manager.NewBatchDeleteWithClient(client).Delete(aws.BackgroundContext(), iter); err != nil {
		exitErrorf("Unable to delete objects from bucket %q, %v", newBucket, err)
	}
	fmt.Printf("All objects in bucket %q are deleted\n", newBucket)

	_, err = client.DeleteBucket(&s3.DeleteBucketInput{
		Bucket: aws.String(newBucket),
	})
	if err != nil {
		exitErrorf("Unable to delete bucket %q, %v", newBucket, err)
	}

	// Wait until bucket is deleted before finishing
	fmt.Printf("Waiting for bucket %q to be deleted...\n", newBucket)

	err = client.WaitUntilBucketNotExists(&s3.HeadBucketInput{
		Bucket: aws.String(newBucket),
	})

	if err != nil {
		exitErrorf("Error occurred while waiting for bucket to be deleted, %v", newBucket)
	}

	fmt.Printf("Bucket %q successfully deleted\n", newBucket)

}

func exitErrorf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}
