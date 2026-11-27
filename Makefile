.PHONY: ui build cross-linux run demo dev-api dev-ui test docker clean

ui:
	npm --prefix web install
	npm --prefix web run build

build: ui
	go build -ldflags="-s -w" -o bin/zetnix ./cmd/zetnix

cross-linux: ui
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/zetnix-linux-amd64 ./cmd/zetnix

run: build
	./bin/zetnix --addr :8080

demo: build
	./bin/zetnix --demo --addr :8080

dev-api:
	mkdir -p web/dist
	touch web/dist/.keep
	go run ./cmd/zetnix --demo --addr :8080

dev-ui:
	npm --prefix web run dev

test:
	go test -count=1 ./...

docker:
	docker build -t zetnix:1.0.0 .

clean:
	rm -rf bin web/dist
