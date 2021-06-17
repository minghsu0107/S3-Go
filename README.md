# S3-Go
Some basic S3 operations using Golang AWS SDK.
## Usage
```
S3_ENDPOINT=http://localhost:9000 \
S3_REGION=us-east-1               \
AWS_ACCESS_KEY_ID=access123       \
AWS_SECRET_KEY=secret123          \
go run main.go
```
Operations are performed in the following order:
- List buckets
- List objects in a bucket with pagination
- Create a bucket
  - s3 will return error if the bucket already exists
- Upload an object to a bucket with public read access and 3 hour expiration, intelligently buffering large files into smaller chunks and sending them in parallel across multiple goroutines
  - s3 will overwrite the existing file when you upload the file with same name
  - Note that the expiration here determines how long the object is cacheable. If you want to delete / archive objects, you should configure the life cycle policy on the bucket level
- Download an object in a bucket
- Create presigned url of an object with 15 minutes expiration
- List objects in a bucket
- Delete an object in a bucket
- Put an object to a bucket
- Delete all objects in a bucket
- Delete a bucket
