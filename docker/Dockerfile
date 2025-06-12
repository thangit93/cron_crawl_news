# Use the official Golang image
FROM golang:1.24.3-alpine

# Set timezone (UTC+7)
RUN apk add --no-cache tzdata && cp /usr/share/zoneinfo/Asia/Ho_Chi_Minh /etc/localtime && echo "Asia/Ho_Chi_Minh" > /etc/timezone

WORKDIR /app
COPY . .
RUN go mod download

RUN go build -o crawler main.go

COPY crontab /etc/crontabs/root
# Start cron in foreground
CMD ["crond", "-f"]