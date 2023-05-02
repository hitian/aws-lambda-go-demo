.PHONY: clean build

clean: 
	rm -rf ./main ./upload.zip
	
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.version=`date +'%Y-%m-%d_%H_%M_%S'`" -o ./main ./src
	mkdir -p geoip && curl --output "geoip/GeoLite2-City.mmdb" "https://media.githubusercontent.com/media/hitian/aws-lambda-go-demo/static/geoip/GeoLite2-City.mmdb"

upload:
	zip -X -r ./upload.zip ./main
	aws lambda update-function-code --function-name $FUNCTION_NAME --zip-file fileb://upload.zip