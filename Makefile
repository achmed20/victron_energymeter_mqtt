
run:
	go run main.go

build:
	echo "Compiling for ARM OS (Venus)"
	GOOS=linux GOARCH=arm go build -o bin/main-linux-arm/victron-mqtt-bridge main.go

copy:
	scp bin/main-linux-arm/victron-mqtt-bridge root@einstein:/data
	scp victron-mqtt-emm.yaml root@einstein:/data