
# URL Shortener
URL Shortener provides link shortening service.
## Technologies:
- Golang
- gRPC
- Swagger
- PostgreSQL
- Redis
## Database:
```postgresql
create table if not exists url_list
(
    id bigint not null
    constraint shorten_id
    primary key,
    original_url varchar(255) not null,
    shorted_url varchar(255) not null,
    clicks int not null
);
```
## Deployment:
To deploy this project run

```
docker-compose up -d
```
## Swagger UI:
Run your app, and browse to [http://localhost:8080/docs/index.html](http://localhost:8080/docs/index.html). You will see Swagger 2.0 Api documents as shown below:

![SwaggerUI](https://user-images.githubusercontent.com/74152283/185743694-8b0b32f1-0ba3-434d-b5e2-eadcdb663f9a.png)