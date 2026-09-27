# Searchly — Uygulama Planı

Case study: **Arama Motoru Servisi**. İki farklı provider'dan (JSON + XML) içerik çekilecek, standart formata dönüştürülüp puanlanacak, veritabanına yazılacak. Bu veriler arama / filtreleme / sıralama / sayfalama sunan bir API ve basit bir dashboard ile sunulacak.

> Değerlendirme notu: "Özelliklerin tamamlanmasından çok, tamamlanan kısmın kaliteli olması önemli." Önce çekirdeği (provider → puanlama → DB → arama API'si) sağlam ve test edilmiş şekilde bitir, bonusları sonra ekle.

---

## 1. Kaynak verinin analizi

| | Provider 1 (JSON) | Provider 2 (XML) |
|---|---|---|
| URL | `https://raw.githubusercontent.com/WEG-Technology/mock/refs/heads/main/v2/provider1` | `.../v2/provider2` |
| Liste | `contents[]` | `feed > items > item[]` |
| Başlık | `title` | `headline` |
| Tür | `type`: `video` | `type`: `video` / `article` |
| Video metrikleri | `metrics.views`, `metrics.likes`, `metrics.duration` ("15:30") | `stats.views`, `stats.likes`, `stats.duration` |
| Metin metrikleri | — (şu an yok) | `stats.reading_time`, `stats.reactions`, `stats.comments` |
| Tarih | `published_at` (RFC3339) | `publication_date` (`2006-01-02`) |
| Etiketler | `tags[]` | `categories > category[]` |
| Sayfalama | `pagination {total, page, per_page}` | `meta {total_count, current_page, items_per_page}` |

**Dikkat edilecek noktalar**
- **ID çakışması:** İki provider'da da `v1`, `v2` var. Benzersiz anahtar `(provider, external_id)` olmalı.
- **Tür isimleri farklı:** `article` → iç modelde `text`. Bilinmeyen tür gelirse logla ve atla, sistemi çökertme.
- **Tarih formatları farklı:** Her iki formatı da parse et, UTC olarak sakla.
- **Sıfıra bölme:** `likes/views` ve `reactions/reading_time` hesaplarında payda 0 ise etkileşim puanı 0 olmalı.
- **Tarihler 2024:** Güncellik puanı bugün için hep 0 çıkacak. Testlerde `now` dışarıdan verilebilir (enjekte edilen `Clock`) olmalı.
- Mock dosyalar statik. Sayfalama meta'sı "150 kayıt" dese de tek sayfa dönüyor. Yine de client sayfalama döngüsünü destekleyecek şekilde yazılır, sayfa boş gelince durur.

---

## 2. Teknoloji tercihleri (README'de gerekçeleriyle yazılacak)

| Alan | Seçim | Gerekçe |
|---|---|---|
| Dil | **Go 1.25** | Hızlı, eşzamanlılık kolay, tek binary. İskelet hazır. |
| HTTP | **Fiber v3** | Günlük kullanılan framework, mülakatta canlı değişiklikte hız. Hazır `requestid`/`recover` middleware'leri, merkezi `ErrorHandler`. |
| Veritabanı | **PostgreSQL 16** | Tutarlılık (ACID), `UPSERT`, dahili full-text search (`tsvector` + `ts_rank`) ile alakalılık skoru |
| DB sürücüsü | `jackc/pgx/v5` (ORM yok) | Full-text search ve upsert gibi Postgres özelliklerini doğrudan kullanmak için. Connection pool dahil. |
| Migration | `golang-migrate/migrate` | Versiyonlu SQL şeması, binary'ye gömülü (`embed`), açılışta otomatik uygulanır |
| Cache | **Redis** | Arama sonuçlarının cache'i, çoklu instance'ta paylaşımlı |
| Rate limit | `golang.org/x/time/rate` | Token bucket, provider başına limit |
| Test | `testing` + `testify` + `testcontainers-go` | Unit + gerçek Postgres ile entegrasyon |
| Dashboard | Go `embed` ile servis edilen tek HTML + vanilla JS | Ayrı build/deploy gerekmez |
| Doküman | OpenAPI 3 (`api/openapi.yaml`) + Swagger UI | API dokümantasyonu şartı |
| Çalıştırma | Docker Compose (app + postgres + redis) | Tek komutla kurulum |

---

## 3. Mimari

```
                 ┌──────────── Sync Worker (ticker, örn. 10 dk) ────────────┐
Provider1 (JSON) ─┤  ProviderClient ─► RateLimiter ─► Parser ─► Normalizer  │
Provider2 (XML)  ─┤                                          │               │
                 │                                   Scorer (formül)        │
                 │                                          ▼               │
                 └──────────────────────────► Repository.Upsert ──► PostgreSQL
                                                                 │
                                                  cache invalidation (Redis)

Client / Dashboard ─► HTTP API ─► SearchService ─► Cache (Redis) ─miss─► Repository ─► PostgreSQL
```

**Katmanlar (dizin eşleşmesi)**

```
cmd/searchly/main.go            # wiring: config, db, redis, providers, worker, server
internal/
  config/                       # env config (DB_URL, REDIS_URL, SYNC_INTERVAL, provider URL'leri, rate limitler)
  model/
    provider.go                 # provider response modelleri + ToContents() → []model.Content
    content.go                  # model.Content, ContentType, Validate
  provider/
    httpclient/                 # rate limit, timeout, 429/retry; ortak HTTP kodu
    jsonprovider/               # Client[T]: GET + json.Unmarshal → T
    xmlprovider/                # Client[T]: GET + xml.Unmarshal → T
  scoring/scorer.go             # puanlama formülü (saf fonksiyonlar, %100 test)
  sync/worker.go                # periyodik çek → normalize → puanla → upsert
  repository/postgres/          # ContentRepository implementasyonu
  cache/redis.go                # SearchCache
  service/search.go             # iş mantığı: cache → repo, validasyon
  handler/                      # HTTP handler'ları, request parse, response DTO, hata mapping
  server/                       # Fiber app, route kaydı + middleware (logging, recover, request-id, CORS, rate limit)
  database/                     # pgx pool + gömülü migration'lar
  web/                          # dashboard (embed edilen static dosyalar)
api/openapi.yaml
docker-compose.yml (kök dizin), Dockerfile
```

**Genişletilebilirlik: generic client, modeller ve dönüşüm domain'de**

`jsonprovider.Client[T]` ve `xmlprovider.Client[T]` sadece isteği atar ve cevabın tamamını `T`'ye decode eder. Response modeli ya da domain dönüşümü bilmezler. Response modelleri ve onları `model.Content`'e çeviren `ToContents()` metodları `internal/model`'de durur:

```go
resp, err := jsonprovider.New[model.Provider1Response](url, httpclient.Config{...}).Fetch(ctx)
contents, skipped := resp.ToContents()
```
Yeni provider = yeni response modeli + `ToContents()` metodu. Generic client'lar değişmez. Özel decode gerekirse (örneğin türe göre değişen `metrics`) modelin kendisi çözer (`json.RawMessage`, `UnmarshalJSON`).

---

## 4. Veri modeli

```sql
CREATE TABLE contents (
    id              BIGSERIAL PRIMARY KEY,
    provider        TEXT        NOT NULL,
    external_id     TEXT        NOT NULL,
    title           TEXT        NOT NULL,
    type            TEXT        NOT NULL CHECK (type IN ('video','text')),
    views           BIGINT      NOT NULL DEFAULT 0,
    likes           BIGINT      NOT NULL DEFAULT 0,
    duration        TEXT,
    reading_time    INT,
    reactions       BIGINT      NOT NULL DEFAULT 0,
    comments        BIGINT      NOT NULL DEFAULT 0,
    tags            TEXT[]      NOT NULL DEFAULT '{}',
    published_at    TIMESTAMPTZ NOT NULL,
    score           DOUBLE PRECISION NOT NULL,   -- final skor (popülerlik)
    search_vector   TSVECTOR GENERATED ALWAYS AS (
                      setweight(to_tsvector('simple', title), 'A') ||
                      setweight(to_tsvector('simple', array_to_string(tags,' ')), 'B')
                    ) STORED,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, external_id)
);
CREATE INDEX idx_contents_search ON contents USING GIN (search_vector);
CREATE INDEX idx_contents_type_score ON contents (type, score DESC);
```

- **Kalıcı veri tutarlılığı:** Her sync tek transaction içinde `INSERT ... ON CONFLICT (provider, external_id) DO UPDATE`. İdempotent, tekrar çalıştırılabilir. Bir provider hata verirse diğerinin verisi yine yazılır, hatalı provider'ın eski verisi korunur.
- **Metrikler ham olarak saklanır:** Formül değişirse DB'den yeniden hesaplanabilir.

---

## 5. Puanlama algoritması (`internal/scoring`)

```
Final = (Temel * TürKatsayısı) + Güncellik + Etkileşim

Temel      video: views/1000 + likes/100        text: reading_time + reactions/50
Katsayı    video: 1.5                           text: 1.0
Güncellik  ≤7 gün: 5 | ≤30 gün: 3 | ≤90 gün: 1 | daha eski: 0
Etkileşim  video: (likes/views)*10              text: (reactions/reading_time)*5
```

- Katsayılar ve güncellik eşikleri config'ten gelir (hardcode değil). Varsayılanlar case'tekiler.
- `Score(c Content, now time.Time) float64`: saf fonksiyon, yan etkisi yok.
- Güncellik zamanla değiştiği için skor **her sync'te yeniden hesaplanır**.
- Örnek kontrol (provider1/v1, eski tarih): `(15000/1000 + 1200/100)*1.5 + 0 + (1200/15000)*10 = 40.5 + 0.8 = 41.3`

---

## 6. API tasarımı

### `GET /api/v1/contents/search`

| Param | Tip | Varsayılan | Açıklama |
|---|---|---|---|
| `q` | string | — | Anahtar kelime (başlık + etiketlerde aranır). Boşsa hepsi listelenir. |
| `type` | `video` \| `text` | — | Tür filtresi |
| `sort` | `popularity` \| `relevance` | `q` varsa `relevance`, yoksa `popularity` | Sıralama |
| `page` | int ≥1 | 1 | Sayfa |
| `per_page` | int 1–100 | 20 | Sayfa boyutu |

- **popularity:** `ORDER BY score DESC, id`
- **relevance:** `ts_rank(search_vector, query)` ile metin eşleşmesi, eşitlikte `score`. `q` yokken relevance istenirse popularity'e düşülür.
- Prefix araması için `plainto_tsquery` yerine kelimeler `word:*` şeklinde birleştirilir. Kullanıcı girdisi temizlenir, SQL tamamen parametreli.

```json
{
  "data": [
    { "id": 12, "provider": "provider1", "title": "Go Programming Tutorial",
      "type": "video", "score": 41.3, "relevance": 0.61,
      "published_at": "2024-03-15T10:00:00Z", "tags": ["programming","tutorial"] }
  ],
  "pagination": { "page": 1, "per_page": 20, "total": 8, "total_pages": 1 }
}
```

### Diğer endpoint'ler
- `GET /api/v1/contents/{id}`: tek içerik + skor kırılımı (temel/güncellik/etkileşim). Dashboard'da detay için.
- `POST /api/v1/sync`: manuel senkronizasyon tetikleme (bonus).
- `GET /healthz`: canlılık. `GET /readyz`: DB + Redis bağlantısı.
- `GET /docs`: Swagger UI. `GET /`: dashboard.

### Hata formatı (tek tip)
```json
{ "error": { "code": "invalid_parameter", "message": "per_page must be between 1 and 100" } }
```
400 validasyon, 404 bulunamadı, 429 rate limit, 500 iç hata. İç hata detayı loglanır, client'a sızdırılmaz.

---

## 7. Provider entegrasyonu ve istek limiti

- **Rate limit:** Her provider için ayrı `rate.Limiter` (örn. 5 istek/sn, burst 1). İstekten önce `limiter.Wait(ctx)`.
- **Timeout:** `http.Client{Timeout: 5s}` + context.
- **Retry:** 5xx ve ağ hatalarında 3 deneme, exponential backoff + jitter. 429 gelirse `Retry-After`'a uyulur. 4xx'te retry yapılmaz.
- **Hata izolasyonu:** Provider'lar `errgroup` ile paralel çekilir. Birinin hatası diğerini durdurmaz, hatalar loglanır.
- **Sync zamanlaması:** Başlangıçta bir kez, sonra `SYNC_INTERVAL` (vars. 10 dk) ile. `ctx` iptal edilince (shutdown) temiz çıkış yapılır.
- **Gelen istekler için rate limit (bonus):** IP bazlı token bucket middleware, aşımda 429.

---

## 8. Cache stratejisi

- **Ne cache'lenir:** Arama yanıtları. Anahtar `search:v{ver}:{sha1(normalize edilmiş query params)}`, TTL 5 dk.
- **Invalidation:** Sync başarıyla veri yazdığında `search:version` sayacı `INCR` edilir. Eski anahtarlar kendiliğinden erişilemez olur ve TTL ile silinir. `KEYS`/`SCAN` gerekmez.
- **Dayanıklılık:** Redis erişilemezse cache atlanır, doğrudan DB'ye gidilir (fail-open) ve uyarı loglanır. Cache hiçbir zaman doğruluk kaynağı değildir.
- Arkasında `SearchCache` interface'i var. Testlerde in-memory implementasyon kullanılır.
- README'de ek öneri: yüksek trafikte sık sorgular için in-process LRU (L1) + Redis (L2).

---

## 9. Dashboard

- `internal/web/index.html` + `app.js` + `style.css`, `embed.FS` ile `/` altında servis edilir.
- Arama kutusu, tür filtresi (Tümü/Video/Metin), sıralama seçimi (Popülerlik/Alakalılık), sayfalama.
- Tablo kolonları: **Başlık · İçerik türü · Skor** (+ provider, yayın tarihi).
- Aramada debounce (300 ms), boş sonuç ve hata durumları gösterilir.
- Bonus: satıra tıklayınca skor kırılımı (`/contents/{id}`).

---

## 10. Test stratejisi

| Seviye | Kapsam | Araç |
|---|---|---|
| Unit | `scoring`: tablo bazlı testler, sıfıra bölme, güncellik sınırları (7/30/90. gün) | `testing` |
| Unit | JSON/XML parser'lar: `testdata/` altındaki gerçek mock dosyalarla, bozuk girdi, bilinmeyen tür | golden files |
| Unit | Provider client: `httptest.Server` ile retry, 429, timeout senaryoları | `httptest` |
| Unit | `SearchService`: mock repo + mock cache (hit/miss, Redis down) | interface mock |
| Unit | Handler: parametre validasyonu, hata formatı | `httptest.ResponseRecorder` |
| Entegrasyon | Repository: upsert idempotency, arama/filtre/sıralama/sayfalama | `testcontainers-go` (Postgres) |
| E2E (smoke) | docker-compose up → sync → `/search` | `test/` + Makefile hedefi |

`make test` unit testleri, `make test-integration` Docker gerektirenleri çalıştırır (`//go:build integration`). CI'da ikisi de koşar.

---

## 11. Uygulama adımları (sırayla)

1. ✅ **Temel altyapı:** config genişletme, Docker Compose (postgres + redis), migration'lar, `/readyz`.
2. **Puanlama:** `scoring` paketi + kapsamlı unit testler. Ona bağımlı bir şey olmadığı için ilk bitirilecek.
3. ✅ **Provider'lar:** generic `jsonprovider`/`xmlprovider` client'ları + `httpclient`, `content` tarafında response modelleri ve dönüşümler. (Henüz `main`'e bağlı değil.)
4. **Repository:** Postgres upsert + arama sorgusu + entegrasyon testleri.
5. **Sync worker:** periyodik çalıştırma, hata izolasyonu, cache versiyon artırma.
6. **Search service + API:** handler, validasyon, hata formatı, sayfalama, cache entegrasyonu.
7. **Middleware:** request-id, yapılandırılmış log (`slog`), panic recover, CORS, inbound rate limit.
8. **Dashboard:** embed edilen statik arayüz.
9. **Dokümantasyon:** `api/openapi.yaml` + Swagger UI, README (kurulum, mimari kararlar, teknoloji gerekçeleri, örnek `curl`'lar).
10. **Cilalama (bonus):** Dockerfile (multi-stage), GitHub Actions (lint + test), `golangci-lint`, Prometheus `/metrics`.

---

## 12. Teslim kontrol listesi

- [ ] GitHub reposu public. **Repo adında, kodda ve README'de "Enuygun" adı geçmiyor.**
- [ ] `go.mod` modül yolu gerçek repo yoluna güncellendi (örn. `github.com/<kullanıcı>/searchly`)
- [ ] `docker compose up` ile tek komutta ayağa kalkıyor
- [ ] README: dil seçimi, mimari kararlar, teknoloji gerekçeleri, kurulum, cache önerisi, bilinen kısıtlar
- [ ] API dokümantasyonu (OpenAPI + Swagger UI)
- [ ] `make test` yeşil, puanlama ve parser'lar iyi test edilmiş
- [ ] Anlamlı, küçük commit'ler
