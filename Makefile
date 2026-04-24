run:
	go run ./cmd/sidecar

fix:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run --fix
