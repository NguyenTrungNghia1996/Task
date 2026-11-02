.PHONY: dev run swag build seed test

# Chạy với Hot Reload
dev:
	air

# Chạy trực tiếp không qua Air
run:
	go run cmd/api/main.go

# Tạo lại tài liệu Swagger OpenAPI
swag:
	swag init -g cmd/api/main.go --parseDependency --parseInternal

# Nạp dữ liệu mẫu
seed:
	go run cmd/seed/main.go

# Build file thực thi độc lập
build:
	go build -o bin/api cmd/api/main.go

# Chạy unit tests
test:
	go test -v ./...

