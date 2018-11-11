.PHONY: clean build

clean: 
	rm -rf ./main ./upload.zip
	
build:
	GOOS=linux GOARCH=amd64 go build -o ./main ./src

upload:
	zip -X -r ./upload.zip ./main
	aws lambda update-function-code --function-name $FUNCTION_NAME --zip-file fileb://upload.zip