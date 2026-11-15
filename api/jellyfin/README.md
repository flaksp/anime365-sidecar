# Генерация клиента Jellyfin

`openapi.json` — локальная копия [официальной стабильной спецификации Jellyfin](https://api.jellyfin.org/openapi/jellyfin-openapi-stable.json). Версия API указана в `info.version`. Генерация работает с локальной копией и не загружает спецификацию из сети.

Из корня репозитория выполните:

```sh
make generate-jellyfin-client
```

Цель запускает актуальный `oapi-codegen` через `go run ...@latest`; устанавливать генератор или добавлять его в `go.mod` не нужно. Список операций находится в `oapi-codegen.yaml`. В `pkg/jellyfinclient/client.gen.go` хранятся только сгенерированные модели и клиент. Этот файл хранится в репозитории и не редактируется вручную; код адаптера находится в `internal/jellyfin`, настройка HTTP-клиента — в `cmd/sidecar/module/jellyfinclient.go`.

Чтобы обновить спецификацию и клиент:

```sh
make update-jellyfin-openapi
make generate-jellyfin-client
make fix
go build ./...
```

После обновления проверьте изменения спецификации и клиента. Как и для Emby, сгенерированные файлы исключены из диагностики и автоисправлений правилом `generated: strict`.
