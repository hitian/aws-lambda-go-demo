.PHONY: clean build

clean: 
	rm -rf ./main ./upload.zip
	
build:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.version=`date +'%Y-%m-%d_%H_%M_%S'`" -o ./main ./src

upload:
	zip -X -r ./upload.zip ./main
	aws lambda update-function-code --function-name $FUNCTION_NAME --zip-file fileb://upload.zip