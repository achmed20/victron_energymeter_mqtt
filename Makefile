build:
	go build -o bin/victron-mqtt-bridge main.go

run:
	go run main.go

compile:
	echo "Compiling for ARM OS (Venus)"
	GOOS=linux GOARCH=arm go build -o bin/main-linux-arm/victron-mqtt-bridge main.go
