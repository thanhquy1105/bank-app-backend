# Build stage
FROM golang:1.20-alpine3.17 AS builder
WORKDIR /app
COPY . .
RUN go build -o main main.go
RUN apk add curl
RUN curl -L https://github.com/gobuffalo/pop/releases/download/v6.1.1/pop_6.1.1_linux_amd64.tar.gz | tar xvz

# Run stage
FROM alpine:3.17
WORKDIR /app
COPY --from=builder /app/main .
COPY --from=builder /app/soda ./soda
COPY app.env .
COPY start.sh .
COPY wait-for.sh .
COPY db/migrations ./migrations
COPY db/database.yml ./database.yml

EXPOSE 8080
CMD [ "/app/main" ]
ENTRYPOINT [ "/app/start.sh" ]

