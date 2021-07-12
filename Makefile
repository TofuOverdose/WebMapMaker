build:
	go build -o bin/makemap cli/main.go
	echo "Compiled binary bin/makemap"

run:
	go run cli/main.go