## Build app
```
docker compose build
```

## Run app with crond
```
docker compose up -d
```
## Run app without crond
```
docker compose exec app sh
./crawler
```
