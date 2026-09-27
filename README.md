# Searchly

## Dizin yapısı

| Dizin | Açıklama |
|---|---|
| `cmd/searchly` | Uygulamanın giriş noktası (`main.go`) |
| `internal/config` | Ortam değişkenlerinden konfigürasyon |
| `internal/server` | Fiber uygulaması, route tanımları ve middleware'ler |
| `internal/handler` | HTTP handler'ları |
| `internal/service` | İş mantığı katmanı |
| `internal/repository` | Veri erişim katmanı |
| `internal/database` | pgx bağlantı havuzu ve gömülü SQL migration'ları |
| `api` | OpenAPI / proto tanımları |
| `configs` | Örnek konfigürasyon dosyaları |
| `scripts` | Yardımcı script'ler |
| `deployments` | Docker / Kubernetes dosyaları |
| `test` | Entegrasyon testleri |
| `docs` | Dokümantasyon |

## Komutlar

```sh
make db-up  # Postgres'i başlat (localhost:5433)
make run    # sunucuyu başlat (varsayılan port 8080)
make build  # bin/searchly üret
make test   # testleri çalıştır
make lint   # go vet
```

Sağlık kontrolü: `curl localhost:8080/healthz` (canlılık), `curl localhost:8080/readyz` (DB erişimi)

Migration'lar uygulama açılırken otomatik uygulanır.
