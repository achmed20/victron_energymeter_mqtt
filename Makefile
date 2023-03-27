
run:
	go run main.go

build:
	echo "Compiling for ARM OS (Venus)"
	GOOS=linux GOARCH=arm go build -o bin/main-linux-arm/victron-mqtt-bridge main.go
	mkdir -p .build
	cp bin/main-linux-arm/victron-mqtt-bridge .build
	cp ./assets/* .build

copy:
	scp .build/* root@einstein:/data
