# URL Shortener

## Installation
Cloning the repo :
```
git clone https://github.com/ducan172002/URLShortener.git
```
## Run

Run with Docker

```bash
docker-compose up
```

## Using CURL
### Home page
```
curl http://localhost:8080/shorten
```

Response:
`{"message": "Welcome to URL shortener"}`

### Generate shortener
```
curl -H "Content-Type: application/json" -X POST -d '{"long_url":"https://www.google.com/"}' http://localhost:8080/shorten
```
Response:
`{
    "response": "http://localhost:8080/GTGV4",
    "status": "successful"
}`

### Redirect
Open url in your browser [http://localhost:8080/GTGV4](http://localhost:8080/GTGV4) OR
```
curl http://localhost:8080/bNGTGV4
```

### Get info for url shortener
```
curl http://localhost:8080/info/GTGV4 
```

Response:
```
{
    "long_url": "https://www.google.com/",
    "short_url": "http://localhost:8080/GTGV4",
    "id": 239591655106702,
    "click": 0,
    "create_at": "2022-03-02T05:08:13.099296Z",
    "update_at": "2022-03-02T05:08:13.099296Z"
}
```

### Delete shortener
```
curl -X DELETE http://localhost:8080/GTGV4
```
Response:
`{"message":"delete successful"}`

### Update shortener (change long url)
```
curl -X PUT http://localhost:8080/GTGV4 -d '{"long_url":"https://vn.search.yahoo.com/"}'
```
Response:
`{"message":"update successful"}`
Accessing [http://localhost:8080/GTGV4](http://localhost:8080/GTGV4) will redirect  [https://vn.search.yahoo.com/](https://vn.search.yahoo.com/)
