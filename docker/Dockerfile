# Use the official Golang image
FROM golang:1.24.3-alpine

# Set timezone (UTC+7)
RUN apk add --no-cache tzdata curl && cp /usr/share/zoneinfo/Asia/Ho_Chi_Minh /etc/localtime && echo "Asia/Ho_Chi_Minh" > /etc/timezone

WORKDIR /app

RUN touch /app/crawler.log && touch /app/document.log

COPY . .
RUN go mod download

# Build site crawler
RUN go build -o crawler ./cmd/sites && \
    go build -o document ./cmd/documents

COPY crontab /etc/crontabs/root
# Start cron in foreground
CMD ["crond", "-f"]