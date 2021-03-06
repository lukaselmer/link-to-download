# Link to Download

## Running Locally

```sh
go get -u github.com/lukaselmer/link-to-download
cd $GOPATH/src/github.com/lukaselmer/link-to-download
bin/run
```

## Deploying

```sh
git push heroku master
heroku open
```

## Usage

### Store File

<http://link-to-download.dev:3000/store?api_key={{API_KEY}}&url=https://{{FILE_URL-ending-with-.pdf}}>

Result:

```json
{
  "persistentLink":"https://s3-...",
  "temporaryLink":"http://link-to-download.dev:3000/download/..."
}
```

### Download File

<http://link-to-download.dev:3000/download/{{FILE_NAME}}>
