FROM golang:alpine as builder
WORKDIR /build
ADD . .
RUN GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.version=`date +'%Y-%m-%d_%H_%M_%S'`" -o /go/bin/demo-server ./src

FROM alpine:latest
WORKDIR /app
ENV simpleHTTP=1
ENV PORT=9000
ENV GIN_MODE=release
COPY --from=builder /go/bin/demo-server /app/demo-server
ADD ./geoip /app/geoip

EXPOSE 9000
ENTRYPOINT [ "/app/demo-server" ]
