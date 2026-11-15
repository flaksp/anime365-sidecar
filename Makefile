run:
	go run ./cmd/sidecar

fix:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run --fix

generate-emby-client:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest --config api/emby/oapi-codegen.yaml api/emby/openapi.json

update-emby-openapi:
	curl --fail --location --output api/emby/openapi.json https://raw.githubusercontent.com/MediaBrowser/Emby.SDK/master/Resources/OpenApi/openapi_v3.json
