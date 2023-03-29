
run:
	go run main.go

build:
	echo "Compiling for ARM OS (Venus)"
	GOOS=linux GOARCH=arm go build -o .build/victron-mqtt-bridge main.go
	cp ./assets/* .build

release:
	zip -FSrj build.zip ./.build/* README.md

install:
	@ssh root@einstein "killall victron-mqtt-bridge"
	scp .build/* root@einstein:/data

update:
	ssh root@einstein "killall victron-mqtt-bridge"
	scp .build/victron-mqtt-bridge root@einstein:/data
