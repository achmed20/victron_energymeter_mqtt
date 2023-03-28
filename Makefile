
run:
	go run main.go

build:
	echo "Compiling for ARM OS (Venus)"
	GOOS=linux GOARCH=arm go build -o .build/victron-mqtt-bridge main.go
	cp ./assets/* .build

release:
	zip -FSrj build.zip ./.build/* README.md

copy:
	scp .build/* root@192.168.12.205:/data
