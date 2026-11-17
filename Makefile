run:
	go run ./cmd/sidecar

fix:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run --fix

generate-emby-client:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest --config api/emby/oapi-codegen.yaml api/emby/openapi.json

update-emby-openapi:
	curl --fail --location --output api/emby/openapi.json https://raw.githubusercontent.com/MediaBrowser/Emby.SDK/master/Resources/OpenApi/openapi_v3.json

generate-jellyfin-client:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest --config api/jellyfin/oapi-codegen.yaml api/jellyfin/openapi.json

update-jellyfin-openapi:
	curl --fail --location --output api/jellyfin/openapi.json https://api.jellyfin.org/openapi/jellyfin-openapi-stable.json

generate-anime365-client:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest --config api/anime365/oapi-codegen.yaml api/anime365/openapi.yaml

update-anime365-openapi:
	curl --fail --location --output api/anime365/openapi.yaml https://smotret-anime.org/api/openapi.yaml
