# HTTP Upload
This example shows how to upload a file to S3 via HTTP using Gin.

To run the sample code, you should provide the following environment variables for S3:
```bash
S3_REGION=us-east-1               \
S3_BUCKET=myfilebucket            \
AWS_ACCESS_KEY_ID=access123       \
AWS_SECRET_KEY=secret123          \
go run maing.go
```
The server will listen on port 8088.

![](https://i.imgur.com/bOhHmgz.png)

You could test the upload handler using [Postman](https://www.postman.com/):

![](https://i.imgur.com/Uf3uWS9.png)

Or more commonly, if you are going to upload files via html file upload element (`<input type="file" />`) in web browsers, you could send the request as follows:

```javascript
import Axios from 'axios';

uploadFile = (file) => {
  const formData = new FormData();
  formData.append('file', file);
  Axios.post('http://localhost/myfile', formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  }).then((resp) => {
    if (resp.status === 200) {
      console.log('File uploaded');
    }
  });
};
```
After the file is successfully uploaded, you could go check the file in your bucket. Take AWS S3 for example:

![](https://i.imgur.com/UL3saIQ.png)