FROM alpine:latest
ARG TARGETARCH
WORKDIR /app
ENV simpleHTTP=1
ENV PORT=9000
ENV GIN_MODE=release
COPY --chmod=755 dist/demo-server-linux-${TARGETARCH} /app/demo-server
ADD https://media.githubusercontent.com/media/hitian/aws-lambda-go-demo/static/geoip/GeoLite2-City.mmdb /app/geoip/GeoLite2-City.mmdb

EXPOSE 9000
ENTRYPOINT [ "/app/demo-server" ]
