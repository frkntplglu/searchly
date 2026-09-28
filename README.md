# Searchly

Farklı içerik sağlayıcılardan (provider) gelen videoları ve makaleleri tek bir formatta toplayan, puanlayan ve arama sorgusuna göre en uygun içerikleri sıralayıp sunan bir arama servisi. Üzerinde basit bir web dashboard'u ve interaktif API dokümantasyonu var.

- **Dil:** Go 1.26
- **Veritabanı:** PostgreSQL 16
- **HTTP:** Fiber v3

## İçindekiler

- [Hızlı başlangıç](#hızlı-başlangıç)
- [Yerel geliştirme](#yerel-geliştirme)
- [Yapılandırma](#yapılandırma)
- [API](#api)
- [Case gereksinimleri](#case-gereksinimleri)
- [Mimari](#mimari)
- [Teknoloji tercihleri](#teknoloji-tercihleri)
- [Tasarım kararları](#tasarım-kararları)
- [Cache önerisi](#cache-önerisi)
- [Ölçeklenirken ingest: sayfalama ve artımlı senkronizasyon](#ölçeklenirken-ingest-sayfalama-ve-artımlı-senkronizasyon)
- [Test stratejisi](#test-stratejisi)
- [To be considered](#to-be-considered)

## Hızlı başlangıç

Gereksinim: Docker.

```sh
docker compose up -d --build
```

Bu komut sırasıyla:

1. PostgreSQL'i başlatır ve sağlıklı olmasını bekler.
2. Şemayı `db.sql`'den oluşturur. Şema zaten varsa bu adımı atlar, yani komut tekrar tekrar çalıştırılabilir.
3. Ingest worker'ı başlatır: provider'lardan veriyi hemen çeker, sonra 5 dakikada bir tekrarlar. Worker'dan birden fazla kopya çalıştırılabilir: `docker compose up -d --scale ingest=3` ([Birden fazla ingest worker](#birden-fazla-ingest-worker)).
4. API'yi başlatır.

Ardından:

| | Adres |
|---|---|
| Dashboard | http://localhost:8080 |
| API dokümantasyonu (Swagger UI) | http://localhost:8080/docs |
| Arama API'si | http://localhost:8080/api/v1/contents?q=go |
| Sağlık kontrolü | http://localhost:8080/health |

Alakalılık sıralamasını denemek için dashboard'da `programming` araması yapıp sıralamayı Popülerlik ile Alakalılık arasında değiştirin. Başlığında "Programming" geçen tek içerik ("Go Programming Tutorial"), popülerlikte 4. sıradayken alakalılıkta 1. sıraya çıkar.

Durdurmak için `docker compose down`, verileri de silmek için `docker compose down -v`.

## Yerel geliştirme

Gereksinim: Go 1.26, Docker (sadece PostgreSQL için).

```sh
make db-up          # PostgreSQL'i başlat (localhost:5432)
make migrate        # şemayı db.sql'den oluştur (şema varsa atlar)
make ingest         # provider'lardan veriyi bir kez çek ve kaydet
make run            # API + dashboard (localhost:8080)
```

Diğer komutlar:

```sh
make ingest-worker     # 5 dakikada bir ingest et (Ctrl+C ile durur)
make test              # unit testler
make test-integration  # PostgreSQL gerektiren testler dahil (önce: make db-up)
make lint              # go vet
make build             # bin/searchly
make db-psql           # veritabanı konsolu
make db-reset          # veritabanını sil
```

## Yapılandırma

Ayarlar ortam değişkenlerinden okunur. Proje kökünde bir `.env` dosyası varsa önce o yüklenir. Ortamda tanımlı değişkenler `.env`'dekilerden önceliklidir. Dosya yoksa aşağıdaki varsayılanlar kullanılır, yani `.env` olmadan da her şey çalışır.

| Değişken | Varsayılan | Açıklama |
|---|---|---|
| `PORT` | `8080` | API'nin dinlediği port |
| `DATABASE_URL` | `postgres://searchly:searchly@localhost:5432/searchly?sslmode=disable` | Uygulamanın bağlandığı veritabanı. Docker Compose, container'lar için aşağıdaki `DB_*` değerlerinden kendi URL'ini kurar. |
| `DB_USER`, `DB_PASSWORD`, `DB_NAME` | `searchly` | Docker Compose'daki PostgreSQL'in bilgileri |
| `DB_PORT` | `5432` | PostgreSQL'in host'ta açıldığı port |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | `json` ya da `text` |
| `PROVIDER1_URL`, `PROVIDER2_URL` | Case'teki mock API'ler | Provider adresleri |
| `PROVIDER_RATE_LIMIT` | `2` | Provider başına saniyede en fazla istek. `0` limitsiz. |
| `PROVIDER_TIMEOUT` | `5s` | İstek başına timeout. `0` timeout yok. |
| `PROVIDER_MAX_RETRIES` | `3` | Ağ hatası, 5xx ve 429'da en fazla retry. `0` retry yok. |
| `INGEST_INTERVAL` | `0` (`make ingest` ve `cmd/ingest` için), `5m` (Docker Compose'da) | Ingest'in tekrar aralığı. `0` bir kez çalışıp çıkar. |
| `INGEST_RUN_TIMEOUT` | `2m` | Bir provider turunun en uzun süresi. Worker'da aynı zamanda kiralama süresi: bu süreyi aşan provider'ı başka bir worker devralabilir. En uzun turdan uzun ve 5 saniyeden fazla olmalı. Worker turu, sonucu kira bitmeden yazabilmek için bu sürenin 5 saniye öncesinde keser. |
| `INGEST_POLL_INTERVAL` | `10s` | Worker'ın vadesi gelmiş provider'lara bakma sıklığı. Bir provider en fazla bu kadar gecikmeyle başlar. |

Geçersiz bir değer (ayrıştırılamayan, negatif ya da pozitif olması gerekirken `0`) uyarı loglanıp varsayılanla değiştirilir.

## API

Bütün endpoint'lerin parametreleri, yanıt şemaları ve örnekleri `/docs` adresindeki Swagger UI'da. Spesifikasyon [`api/openapi.yaml`](api/openapi.yaml) dosyasında.

### `GET /api/v1/contents`

| Parametre | Varsayılan | Açıklama |
|---|---|---|
| `q` | | Anahtar kelime. Başlıkta ve etiketlerde aranır. |
| `type` | tümü | `video` ya da `article` |
| `sort` | `q` varsa `relevance`, yoksa `popularity` | `relevance` ya da `popularity` |
| `page` | `1` | |
| `per_page` | `5` | 1–100 |

```sh
curl 'localhost:8080/api/v1/contents?q=docker&type=video&sort=popularity&page=1&per_page=5'
```

```json
{
  "data": [
    {
      "id": 5,
      "provider": "provider2",
      "provider_id": "v1",
      "type": "video",
      "title": "Introduction to Docker",
      "published_at": "2024-03-15T00:00:00Z",
      "tags": ["devops", "containers"],
      "score": 60.81818181818182,
      "metrics": { "views": 22000, "likes": 1800, "duration_sec": 1515 }
    }
  ],
  "pagination": { "page": 1, "per_page": 5, "total": 1, "total_pages": 1 }
}
```

API yanıtları standart bir zarfla döner (`internal/model/response.go`). Sağlık kontrolü (`/health`) bu zarfın dışında, sade `{"status":"ok"}` döner.

| Durum | Şekil |
|---|---|
| Başarılı | `{ "data": ... }` |
| Başarılı, sayfalı | `{ "data": [...], "pagination": { ... } }` |
| Hata | `{ "error": { "code": "...", "message": "..." } }` |

```json
{ "error": { "code": "invalid_parameter", "message": "per_page must be between 1 and 100" } }
```

Beklenmeyen hataların detayı loglanır, istemciye gönderilmez.

## Case gereksinimleri

| Gereksinim | Karşılığı |
|---|---|
| Anahtar kelimeye göre arama | PostgreSQL full-text search, başlık ve etiketlerde ([Arama](#arama-full-text-search-önek-eşleşmesi-ve-alakalılık)) |
| İçerik türüne göre filtreleme | `type=video\|article` |
| Popülerlik ve alakalılık skoruna göre sıralama | `sort=popularity\|relevance` |
| Sayfalama | `page`, `per_page`, yanıtta `total` ve `total_pages` |
| Farklı formatları standart puana çevirme | Her provider adaptörü kendi formatını `model.Content`'e çevirir, puan `model.Content.BaseScore`'da hesaplanır. |
| Türe göre katsayı, etkileşim ve güncellik puanı | Case'teki formül birebir uygulanıyor ([Skor](#skor-sabit-kısım-saklanır-güncellik-sorgu-anında-eklenir)) |
| JSON ve XML provider'lar | `internal/provider/provider1` (JSON), `internal/provider/provider2` (XML) |
| İstek limiti yönetimi | Provider başına token bucket, timeout, 429/5xx'te retry ([Provider entegrasyonu](#provider-entegrasyonu)) |
| Yeni provider eklemeye uygun yapı | Yeni provider = yeni bir adaptör paketi + `cmd/ingest`'te bir satır |
| Verilerin veritabanında saklanması, kalıcı tutarlılık | PostgreSQL, CHECK constraint'ler, transaction'lı upsert ([Veri tutarlılığı](#veri-tutarlılığı)) |
| Cache mekanizması önerisi | [Cache önerisi](#cache-önerisi) |
| Dashboard: başlık, tür, skor, sıralama | `http://localhost:8080` |
| API dokümantasyonu | `/docs` (Swagger UI), `api/openapi.yaml` |
| Test stratejisi | [Test stratejisi](#test-stratejisi) |

## Mimari

```
                    ┌──────────── ingest worker (cmd/ingest) ────────────┐
 provider1 (JSON) ──┤  provider1 adaptörü ─┐                              │
                    │                      ├─► ingest ─► service ─► repository ─► PostgreSQL
 provider2 (XML)  ──┤  provider2 adaptörü ─┘   (5 dk'da bir)              │          ▲
                    └────────────────────────────────────────────────────┘          │
                                                                                     │
 Tarayıcı / istemci ─► API (cmd/searchly): handler ─► service ─► repository ────────┘
```

Veri akışı iki ayrı process'e bölünmüş durumda:

- **Ingest worker** provider'lardan veriyi çeker, standart modele çevirir, puanlar ve veritabanına yazar.
- **API** sadece veritabanından okur. Kullanıcı isteği hiçbir zaman provider'a gitmez. Bu yüzden bir provider yavaşlasa ya da çökse bile arama çalışmaya devam eder, son başarılı veriyle.

### Paketler

| Paket | Sorumluluk |
|---|---|
| `cmd/searchly` | API sunucusu: bağımlılıkları kurar, düzgün kapanmayı yönetir |
| `cmd/ingest` | Ingest: bir kez ya da `-interval` ile periyodik çalışır |
| `cmd/migrate` | Şemayı `db.sql`'den oluşturur |
| `api` | OpenAPI spesifikasyonu ve Swagger UI sayfası (binary'ye gömülü) |
| `internal/model` | Provider'dan bağımsız `Content` modeli, doğrulama ve puan hesabı |
| `internal/provider/httpclient` | Provider'lar için ortak HTTP: rate limit, timeout, retry |
| `internal/provider/provider1`, `provider2` | Provider adaptörleri: istek, decode, `model.Content`'e dönüşüm |
| `internal/ingest` | Provider'ları işler. Biri hata verirse diğerleri devam eder. Worker modunda provider'ları `provider_sync` üzerinden diğer worker'larla paylaşır. |
| `internal/service` | İş kuralları: yazmadan önce doğrulama, sayfalama |
| `internal/repository` | PostgreSQL erişimi: upsert, arama, skor ve alakalılık sorguları |
| `internal/handler` | HTTP handler'ları, parametre doğrulama, tek tip hata formatı |
| `internal/server` | Fiber uygulaması, route'lar, middleware'ler (request id, loglama, panic recovery) |
| `internal/dashboard` | Web arayüzü (HTML + vanilla JS, binary'ye gömülü) |
| `internal/database` | Bağlantı havuzu ve şema kurulumu |
| `internal/config` | Ortam değişkenleri ve `.env` |

### Bağımlılıklar tek yönde

Interface'ler, onları **kullanan** pakette tanımlanıyor, üreten pakette değil. Bu, Go'nun önerdiği yaklaşım:

- `service` ihtiyaç duyduğu repository metodlarını kendi içindeki küçük bir interface ile tanımlar.
- `handler` service için aynısını yapar.
- `ingest` provider'lar ve kayıt için aynısını yapar.

Böylece her katman somut tipler döndürür, bağımlılıklar hep dıştan içe doğru akar (`handler → service → repository → model`). Katmanlar birbirinden bağımsız olarak sahte implementasyonlarla test edilebilir. `model` paketi hiçbir iç pakete bağımlı değil. Provider'ların JSON ve XML formatları kendi adaptörlerinde gizli (unexported), domain'e sızmıyor.

## Teknoloji tercihleri

| Tercih | Gerekçe |
|---|---|
| **Go** | Tek binary, hızlı başlangıç, düşük bellek. Ingest worker ve HTTP sunucusu gibi eşzamanlı işler için standart kütüphane yeterli. Statik tipler, provider formatlarının güvenli dönüşümünü kolaylaştırıyor. |
| **Fiber v3** | Günlük kullandığım framework. Hazır `requestid` middleware'i her isteğe loglarda takip için benzersiz bir kimlik veriyor, `recover` bir handler çökse bile sunucuyu ayakta tutuyor. Merkezi hata yakalayıcısı, bilinmeyen route'ları ve beklenmedik hataları da standart hata formatına çeviriyor. |
| **PostgreSQL** | Ek bir arama motoruna gerek bırakmayan yerleşik full-text search (`tsvector`, `ts_rank`, GIN index). Tutarlılık için ACID transaction'lar, `ON CONFLICT` upsert, enum ve CHECK constraint'ler. |
| **pgx (ORM yok)** | Arama sorgusu full-text search, `ts_rank` ve sorgu anında skor hesabı içeriyor. Bunlar bir ORM'de zaten ham SQL'e dönüşürdü. SQL'i doğrudan yazmak, ne çalıştığını görünür kılıyor. Kullanıcı girdisi her zaman parametre olarak gidiyor. |
| **Tek dosyalık şema (`db.sql`)** | İki tablo var (`contents`, `provider_sync`). Versiyonlu bir migration aracı bu aşamada gereksiz karmaşıklık olurdu. `cmd/migrate` şemayı yalnızca yoksa uyguluyor. |
| **ozzo-validation** | "Tür video ise sadece video metrikleri dolu" gibi koşullu kuralları okunabilir şekilde ifade ediyor ve bütün hatalı alanları birlikte raporluyor. |
| **golang.org/x/time/rate** | Provider başına token bucket rate limit. Standart ve eşzamanlı kullanıma güvenli. |
| **log/slog** | Standart kütüphanede, yapılandırılmış JSON loglar. Ek bağımlılık yok. |
| **Vanilla JS dashboard** | Case basit bir arayüz istiyor. Build adımı ve Node bağımlılığı olmadan binary'ye gömülüyor, aynı porttan servis ediliyor. |
| **Elle yazılmış OpenAPI** | Tek endpoint'lik bir API için kod üretme aracı gereksiz. Elle yazılan spesifikasyon OpenAPI 3'ün ifade gücünü koruyor, örneğin türe göre değişen `metrics` için `oneOf`. |

## Tasarım kararları

### Provider entegrasyonu

- **Adaptör yapısı:** Her provider kendi paketinde bir adaptör: `Name()` ve `Fetch(ctx) ([]model.Content, error)`. Provider'ın JSON/XML şekli (`dto.go`) ve dönüşümü (`mapper.go`) paketin içinde gizli. Dışarıya sadece standart model çıkıyor.
- **Yeni provider eklemek:** Yeni bir adaptör paketi yazıp `cmd/ingest`'teki listeye bir satır eklemek yeterli. Ingest, service, repository ve API değişmez.
- **Rate limit:** Her provider'ın kendi HTTP client'ı ve kendi limiter'ı var (varsayılan 2 istek/sn, `PROVIDER_RATE_LIMIT`). Bir provider'ın limiti diğerini etkilemiyor. Worker'da client bir kez oluşturulup bütün turlarda kullanıldığı için limit turlar arasında da korunuyor. Bir provider aynı anda tek bir worker'da çalıştığı için, worker sayısı artsa da limit aşılmıyor.
  - Bugün bir tur tek bir istek olduğu için limit pratikte devreye girmiyor. Retry'lar arasındaki bekleme (500 ms'den başlayıp artıyor) de limitin istekler arasında bıraktığı süreden kısa değil. Limit, sayfalamalı bir provider'da, bir turun yüzlerce istek olduğu durumda önem kazanır ([Sayfalama](#sayfalama)).
- **Dayanıklılık:** İstek başına 5 sn timeout (`PROVIDER_TIMEOUT`). Ağ hatası, 5xx ve 429'da artan beklemeyle en fazla 3 retry (`PROVIDER_MAX_RETRIES`). 429'da `Retry-After` başlığına uyuluyor. 4xx'te retry yapılmıyor, çünkü sonucu değiştirmez. Context iptal edilince bekleme hemen bitiyor.
- **Hata izolasyonu:**
  - Bir provider hata verirse diğerleri yine işleniyor. Hatalı provider'ın önceki verisi veritabanında korunuyor.
  - Geçersiz bir kayıt (bilinmeyen tür, bozuk tarih ya da süre, eksik alan) atlanıp loglanıyor, aynı cevaptaki diğer kayıtlar yazılıyor.
  - Provider1'de `metrics` alanı türe göre şekil değiştiriyor. Bu yüzden ham (`json.RawMessage`) okunup türe göre ayrıca decode ediliyor. Böylece tek bir kayıttaki tip uyuşmazlığı bütün cevabı düşürmüyor.

### Veri tutarlılığı

- **Tekil kayıt:** `(provider, provider_id)` benzersiz. İki provider'da da `v1` gibi ID'ler var, çakışmıyorlar.
- **Upsert:** Her provider'ın verisi tek bir transaction'da `INSERT ... ON CONFLICT DO UPDATE` ile yazılıyor. Tekrar çalıştırmak güvenli, çift kayıt oluşmuyor. Batch'teki tek bir hatalı kayıt bütün batch'i geri alıyor.
- **İki katmanlı doğrulama:** Kayıtlar yazılmadan önce Go'da doğrulanıyor (`model.Content.Validate`). Ayrıca veritabanında da korunuyor:
  - `content_type` enum
  - "video ise sadece video metrikleri, article ise sadece article metrikleri dolu" CHECK constraint'i

### Birden fazla ingest worker

Worker'lar provider'ları kendi aralarında paylaşır. İş birimi "bütün tur" değil, "bir provider'ın bir turu". Zamanlama her worker'ın kendi ticker'ında değil, veritabanındaki `provider_sync` tablosunda tutuluyor:

- **Kiralama:** Her worker 10 saniyede bir (`INGEST_POLL_INTERVAL`), vadesi gelmiş (`next_run_at <= now()`) ve kimsede olmayan bir provider'ı `UPDATE ... FOR UPDATE SKIP LOCKED` ile kiralar, çalıştırır ve bir sonraki çalışmayı `now() + aralık` olarak yazar. Vadesi gelmiş provider kalmayana kadar devam eder. `SKIP LOCKED` sayesinde eşzamanlı worker'lar birbirini beklemez ve aynı provider iki worker'a verilmez.
- **Worker sayısı sıklığı değiştirmez, kapasiteyi artırır:** Kaç worker olursa olsun her provider 5 dakikada bir çalışır. Provider'lar boşta olan worker'lara dağılır. Provider sayısı artınca worker eklemek turu kısaltır.
- **Çöken worker işi kilitlemez:** Kiralama süreli (`locked_until`, `INGEST_RUN_TIMEOUT`, varsayılan 2 dk). Süresi dolan provider'ı başka bir worker alır. Çalışma, sonucu kira bitmeden yazabilmek için bu süreden 5 saniye önce kesilir. Her kiralama yeni bir `claim_token` alır, işi bitirme de bu token'la eşleşir. İş bu arada yeniden kiralanmışsa eski çalışma sonucu yazmaz (`WHERE claim_token = $token`), sadece uyarı loglar. Aynı adla çalışan iki worker da bu yüzden birbirinin işine karışmaz. Nadiren iki worker aynı provider'ı birlikte çalıştırabilir. Upsert tekrar çalıştırılabilir olduğu için veri bozulmaz.
- **Kapanışta iş devredilir:** `SIGTERM` ile yarıda kalan bir provider'ın kilidi bırakılır ve hemen vadesi gelmiş olarak işaretlenir. Başka bir worker kira süresini beklemeden devralır.
- **Durum görünür:** `last_success_at` ve `last_error` her provider'ın son sonucunu gösterir.
- **`make ingest` koordinasyonsuz:** Tek seferlik çalışma bütün provider'ları hemen işler, `provider_sync`'e bakmaz.

Redis ya da bir mesaj kuyruğu yerine PostgreSQL kullanıldı, çünkü zaten var ve bu ölçekte `SKIP LOCKED` yeterli bir iş kuyruğu. Tek bir global advisory lock ise aynı anda tek worker çalıştırır, işi dağıtmaz.

### Yayın tarihleri gün hassasiyetinde saklanır

Provider'lar yayın tarihini farklı hassasiyette veriyor:

| Provider | Örnek | Hassasiyet |
|---|---|---|
| provider1 (JSON) | `2024-03-15T10:00:00Z` | saniye |
| provider2 (XML) | `2024-03-15` | gün |

Güncellik puanı yayın tarihine göre hesaplanıyor. Tarihler olduğu gibi saklansaydı, provider2'nin içerikleri her zaman günün başında (00:00) yayınlanmış sayılırdı. Aynı gün yayınlanan iki içerikten provider2'deki, provider1'deki içerikten saatler önce yayınlanmış gibi görünürdü. Bu da eşik sınırlarında (örneğin tam 7. gün) aynı günün içeriklerinin farklı puan almasına yol açabilirdi.

İki provider'a eşit davranmak için provider1'in zaman damgası UTC'ye çevrilip gün kısmına indiriliyor. Puanlama gün bazında olduğu için saat bilgisinin kaybı sonucu etkilemiyor.

### Skor: sabit kısım saklanır, güncellik sorgu anında eklenir

Case'teki formül:

```
Final Skor = (Temel Puan × İçerik Türü Katsayısı) + Güncellik Puanı + Etkileşim Puanı
```

Bu formül iki farklı hızda değişen parçadan oluşuyor:

- **Sabit kısım** `(temel puan × tür katsayısı) + etkileşim puanı` yalnızca metrikler değiştiğinde değişir. Upsert sırasında Go'da (`model.Content.BaseScore`) hesaplanıp `base_score` kolonuna yazılır. Sıfıra bölme durumları (0 görüntülenme, 0 okuma süresi) etkileşim puanını 0 yapar.
- **Güncellik puanı** veri değişmese de zamanla değişir. Saklansaydı bayatlardı. Bu yüzden arama sorgusunda, içeriğin UTC takvim günü cinsinden yaşına göre SQL'de eklenir (7 gün içinde +5, 30 gün içinde +3, 90 gün içinde +1). Sıralama da bu ifadeye göre yapılır.

Böylece skor her an doğru olur ve zamanlanmış bir işe ihtiyaç duyulmaz. Bedeli, skora göre sıralamanın bir index'ten yararlanamamasıdır.

**Ölçek büyürse:** Güncellik puanı bir UTC günü boyunca sabit kalır. Bu yüzden tam skor bir `score` kolonunda tutulup günde bir kez (ya da her ingest'te) tek bir `UPDATE` ile yenilenebilir ve bu kolon index'lenebilir.

**Not:** Formül case'teki haliyle uygulandığında article'lar video'lardan belirgin şekilde yüksek puan alıyor. Mock veride bir article 298, videolar 33–70 arasında. Sebep article'ın etkileşim puanı: `(reactions / reading_time) × 5`. Bu formülün doğal sonucu, bir hata değil. Formül değiştirilmedi.

### Arama: full-text search, önek eşleşmesi ve alakalılık

Arama, PostgreSQL full-text search ile yapılıyor. Başlıktan (ağırlık `A`) ve etiketlerden (ağırlık `B`) iki vektör otomatik üretiliyor, ikisi de GIN index'li:

- `search_vector` (`english`): Kelimeler köklerine indirilir, böylece "tips" → "Tips", "pattern" → "Patterns" eşleşir.
- `search_vector_simple` (`simple`): Kelimeler olduğu gibi saklanır. Kökünden uzun yarım kelimeler ("concurren" → "concurrency", kökü `concurr`) buradan eşleşir.

Bir içerik iki vektörden birinde eşleşirse sonuçlara girer.

- **Önek eşleşmesi:** Aranan her kelime önek olarak eşleşiyor ("concur" → "concurrency"). Böylece kullanıcı yazarken sonuç görüyor. Birden fazla kelime yazılırsa hepsinin geçmesi gerekiyor.
- **Alakalılık:** `sort=relevance` sonuçları `ts_rank` ile sıralıyor:
  - Tam kelime eşleşmesi önek eşleşmesinden iki kat değerli. Böylece "go" aramasında "Go" geçen içerik "Google" geçenin üstünde çıkıyor.
  - Önek puanı için iki vektörden yüksek olanı alınıyor. Toplanmıyor, çünkü toplansaydı iki vektörde birden eşleşen yarım kelimeler çift puan alırdı.
  - Başlıktaki eşleşme etiketteki eşleşmeden daha değerli. Eşitlikte skor belirliyor.
- **Popülerlik:** `sort=popularity` sonuçları skora göre sıralıyor.
- **Güvenlik:** Sorgu Go'da kuruluyor. Girdiden sadece harf ve rakamlar alınıyor, böylece özel karakterler sorguyu bozamıyor. Sorgu her zaman parametre olarak gönderiliyor.

**Bilinçli takas:** PostgreSQL'in `websearch_to_tsquery` fonksiyonu `or`, `-kelime` ve `"tırnaklı ifade"` sözdizimini destekliyor, ama önek eşleşmesini desteklemiyor. Yazarken arama, bu ileri seviye sözdiziminden daha değerli görüldü.

## Cache önerisi

Cache uygulanmadı. Case bir öneri istiyor, ve mevcut veri boyutunda cache'in faydası ölçülemiyor. Trafik arttığında uygulanacak tasarım şu:

**Ne cache'lenir:** Kullanıcıların arama yanıtları. Provider istekleri değil, onlar zaten ingest'te periyodik yapılıyor. Her arama şu an veritabanına bir full-text search, bir sayım sorgusu ve skor hesabı olarak gidiyor. Yoğun trafikte aynı aramalar ("go", ilk sayfa, filtresiz liste) tekrar tekrar yapılır.

**Neden uygun:** Veri **sadece ingest'te** değişiyor. İki ingest arasında aynı aramanın sonucu hep aynı.

**Neden Redis (in-memory değil):** Ingest worker ve API ayrı process'ler. API'nin içindeki bir in-memory cache'e ingest worker ulaşamaz ve yeni veri yazıldığında onu geçersiz kılamaz, geriye sadece TTL'i beklemek kalır. Redis ikisinin de erişebildiği ortak bir yer. Birden fazla API instance'ı da aynı cache'i paylaşır.

**Tasarım:**

1. **Anahtar:** `search:v{versiyon}:{normalize edilmiş parametrelerin hash'i}`. Parametreler `q`, `type`, `sort`, `page` ve `per_page`. Değer, yanıt sayfasının JSON'u.
2. **Geçersiz kılma:** Ingest başarıyla bitince worker `INCR search:version` çalıştırır. Yeni istekler yeni versiyonla anahtar üretir, eski kayıtlara bir daha erişilmez. Anahtar taramaya ya da silmeye gerek kalmaz.
3. **TTL (örneğin 5 dk):** Güvenlik ağı olarak. Eski versiyonun kayıtları kendiliğinden temizlenir. Güncellik puanının gece yarısı değişmesi de en fazla TTL kadar gecikir.
4. **Fail-open:** Redis erişilemezse cache atlanır, istek doğrudan veritabanına gider ve bir uyarı loglanır. Cache hiçbir zaman doğruluk kaynağı değildir.
5. **Yeri:** Service katmanında. Cache interface'i service'te tanımlanır (tüketici tarafı). Handler ve repository değişmez.

## Ölçeklenirken ingest: sayfalama ve artımlı senkronizasyon

Mevcut ingest mock provider'lara göre tasarlandı: tek bir istek, bütün veri, her turda baştan yazma. Gerçek ve büyük bir provider'da iki şeyin değişmesi gerekir.

### Sayfalama

Mock cevaplar sayfalama bilgisi taşıyor (provider1: `pagination.total/page/per_page`, provider2: `meta.total_count/current_page/items_per_page`). Ama `?page=2` isteğine de aynı dört kaydı dönüyorlar. Meta'ya güvenilseydi aynı sayfa 15 kez çekilirdi. Bu yüzden sayfalama uygulanmadı. Gerçek bir provider'da tasarım şöyle olurdu:

- **Sayfaları adaptör dolaşır.** Sayfalama provider'ın formatının bir parçası, bu yüzden adaptörün içinde kalır. Dışarıya yine sadece `model.Content` çıkar.
- **Sayfa sayfa yazılır.** `Fetch`, bütün veriyi bellekte toplamak yerine sayfaları sırayla verir. Go 1.23'teki range-over-func ile örneğin `iter.Seq2[[]model.Content, error]` olarak. Ingest her sayfayı geldikçe upsert eder. Provider 100 bin kayıt dönse bile bellek kullanımı sabit kalır.
- **Durma koşulu sadece meta'ya bırakılmaz.** Dönen kayıt sayısı sayfa boyutundan azsa, sayfa boşsa ya da `sayfa × sayfa boyutu ≥ toplam` ise durulur. Ayrıca bir üst sayfa sınırı konur. Mock'taki "her sayfada aynı veri" durumu gibi hatalı davranışlar sonsuz döngüye dönüşmez.
- **Rate limit kendiliğinden işler.** Sayfa istekleri aynı client'tan geçtiği için limiter zaten provider başına uygulanıyor. Tur süresinin ingest aralığından kısa kalması izlenir: 500 sayfa, 2 istek/sn'de 250 saniye sürer.
- **Offset yerine cursor tercih edilir.** Provider destekliyorsa (`next_cursor` gibi), sayfalar arasında veri eklenince kayıtların kayması ya da tekrar gelmesi önlenir.
- **Yarıda kalan tur güvenlidir.** Bir sayfa retry'lara rağmen başarısız olursa o provider'ın turu durur. Yazılmış sayfalar kalır, çünkü upsert tekrar çalıştırılabilir. Tur "eksik" olarak işaretlenir. Aşağıdaki silme tespiti eksik turlarda yapılmaz.

### Toplu yazma

Upsert şu an bir provider'ın bütün kayıtlarını tek bir pgx batch'i olarak, tek bir transaction'da yazıyor. Kayıtlar tek bir ağ gidiş-dönüşünde gönderiliyor. Bu, on binlere kadar yeterli. Bir tur çok daha büyük veri getirirse batch parçalara bölünür, örneğin 1.000 kayıtlık gruplar halinde ve her grup kendi transaction'ında:

- Transaction'lar kısa kalır, kilitler uzun süre tutulmaz.
- Bir kayıttaki hata sadece kendi grubunu geri alır, turun geri kalanı yazılır.
- Upsert tekrar çalıştırılabilir olduğu için yarıda kalan bir tur güvenle yeniden denenir.

Daha da büyük hacimlerde her kayıt için ayrı ifade yerine tek ifadeli toplu upsert (`unnest` ile dizi parametreleri) ya da `COPY` ile geçici tabloya yükleyip oradan birleştirme kullanılır.

### Full refresh yerine artımlı senkronizasyon

Şu an her tur bütün kayıtları çekip hepsini yeniden yazıyor. Bunun iki sonucu var: değişmemiş satırlar da her 5 dakikada bir güncelleniyor, ve provider'dan silinen bir içerik veritabanında sonsuza kadar kalıyor. Seçilecek yöntem provider'ın ne desteklediğine bağlı:

**1. Full fetch + yerelde fark bulma** (provider değişiklik bilgisi vermiyorsa)

Her turda bütün veri yine çekilir, ama sadece değişenler yazılır:

- Upsert `ON CONFLICT ... DO UPDATE ... WHERE (kolonlar) IS DISTINCT FROM (EXCLUDED.kolonlar)` ile yapılır. Değişmeyen satır yazılmaz, `updated_at` gerçekten değişim zamanını gösterir.
- Turun başında bir zaman damgası alınır ve görülen her satırın `last_seen_at` alanı güncellenir. Tur **eksiksiz** biterse, o provider'ın bu turda görülmeyen kayıtları silinmiş kabul edilir. Arama dışı bırakılacak şekilde işaretlenir (soft delete) ya da silinir. Eksik bir turda bu yapılmaz, yoksa provider'ın geçici bir hatası bütün verisini sildirebilir.
- Bedeli: Ağ trafiği azalmaz, her turda bütün veri yine indirilir. Görüntülenme ve beğeni sayıları sürekli değiştiği için, gerçek hayatta satırların önemli bir kısmı zaten her turda değişmiş olabilir.
- Ucuz bir ek: Provider `ETag` ya da `Last-Modified` dönüyorsa, `If-None-Match` ile gönderilen istek veri değişmediğinde `304` alır ve tur neredeyse bedavaya biter.

**2. Base + provider delta** (provider değişiklik sorgusu destekliyorsa, örneğin `updated_since` ya da bir değişiklik akışı)

- **Delta turları:** Sık yapılır, sadece son başarılı turdan beri değişenleri çeker. Her provider için son başarılı işaret (zaman damgası ya da cursor) bir `sync_state` tablosunda tutulur. Saat farklarına karşı işaret biraz geriden alınır, örneğin 1 dakikalık örtüşmeyle. Aynı kaydın iki kez gelmesi sorun değil, çünkü upsert tekrar çalıştırılabilir.
- **Base turu:** Seyrek yapılır, örneğin gece bir kez. Bu, yöntem 1'deki gibi eksiksiz bir full fetch'tir. Delta'da kaçan değişiklikleri ve silmeleri düzeltir. Provider silmeleri "tombstone" kaydı olarak bildiriyorsa, silmeler delta'dan da işlenebilir.
- Bedeli: Provider desteği gerekir ve durum (`sync_state`) yönetimi eklenir. Karşılığında ağ trafiği ve yazma yükü, toplam veri boyutuyla değil değişim miktarıyla orantılı olur.

**Özetle:** Provider değişiklik bilgisi vermiyorsa yöntem 1, veriyorsa yöntem 2. Her iki yöntemde de silme tespiti sadece eksiksiz bir full fetch'e dayanır.

## Test stratejisi

Testler, en çok hata çıkabilecek yerlere yoğunlaştırıldı:

| Katman | Ne test ediliyor | Nasıl |
|---|---|---|
| `model` | Skor formülü (elle hesaplanmış değerlerle), sıfıra bölme, doğrulama kuralları | Tablo bazlı unit testler |
| `provider/httpclient` | Retry (5xx, 429, 4xx'te retry yok), `Retry-After`, rate limit, paylaşılan limiter, timeout, context iptali | `httptest` sunucusu |
| `provider/provider1`, `provider2` | Gerçek mock verinin decode'u ve dönüşümü, bozuk kayıtların atlanması, tarih ve süre dönüşümü, HTTP ve decode hataları | `testdata/` altındaki gerçek mock cevaplar |
| `ingest` | Bir provider ya da kayıt hatasında diğerlerinin devam etmesi, vadesi gelince tekrar çalışma, birden fazla worker'da her provider'ın bir kez çalışması, kapanışta yarım kalan provider'ın devredilmesi | Sahte provider, store ve scheduler |
| `handler` | Parametre doğrulama, varsayılanlar, yanıt şekli, tek tip hata formatı | Sahte service, `app.Test` |
| `server` | Panic → 500 (panic mesajı kullanıcıya gitmeden), JSON 404, dashboard ve dokümantasyon servisi | `app.Test` |
| `repository` | Full-text search (kök bulma, yarım kelime, çok kelime, özel karakterler), alakalılık ve popülerlik sıralaması, bütün güncellik eşikleri, upsert, CHECK constraint ihlalinde bütün batch'in geri alınması, provider kiralama (eşzamanlı claim'de tek kazanan, süresi dolan kiranın devralınması, sonucun kaydı) | **Gerçek PostgreSQL**, şema her testte `db.sql`'den kuruluyor |

- **Repository testleri** `integration` build tag'i arkasında (`make test-integration`). Test edilen şey SQL'in kendisi olduğu için sahteyle test edilemez. Geliştirme veritabanına dokunmamak için ayrı bir `searchly_test` veritabanı kullanıyorlar.
- **Service katmanı** repository ile handler arasında ince bir geçiş olduğu için ayrıca test edilmiyor.

## To be considered

Bilinen ve bilinçli olarak bırakılmış durumlar:

- **Kelime içermeyen arama her şeyi döndürür.** Sadece sembollerden oluşan bir arama (`&& !!`) temizlendikten sonra boş kalır ve filtre uygulanmaz. Boş bir aramayla aynı şekilde bütün içerikler listelenir.
- **Stop word'ler sadece yarım kelime olarak eşleşir.** `english` yapılandırması "the", "a" gibi kelimeleri yok sayar. Bu yüzden "the" araması `simple` vektörde "the" ile başlayan kelimeleri ("theory", "them") bulur, "The" kelimesinin kendisini alakalılıkta öne çıkarmaz.
- **Önek eşleşmesi kısa kelimelerde geniş sonuç verir.** "go" araması "Google" gibi "go" ile başlayan kelimeleri de bulur. Tam eşleşmeler alakalılık sıralamasında önde tutulduğu için bunlar listenin altında kalır.
- **`or`, `-kelime` ve `"tırnaklı ifade"` sözdizimi desteklenmez.** Önek eşleşmesi için bilinçli olarak bırakıldı. Bu karakterler sıradan ayraç olarak yok sayılır.
- **Tek bir provider bölünmez.** Worker'lar provider'ları paylaşır ama bir provider'ın turu tek worker'da çalışır. Çok büyük tek bir provider için iş birimi "provider + sayfa aralığı" ya da cursor segmenti olarak küçültülebilir.
- **Mevcut veritabanı otomatik güncellenmez.** `cmd/migrate` şema varsa atlıyor. `provider_sync` tablosundan önce oluşturulmuş bir veritabanı için `docker compose down -v` (ya da `make db-reset`) gerekir. Veri bir sonraki ingest'te yeniden gelir.
- **Sayfalama ve artımlı ingest uygulanmadı.** Mock provider'lar sayfalama bilgisi döndürüyor ama her sayfa isteğine aynı veriyi dönüyor. Ingest her turda bütün veriyi baştan çekip yazıyor ve provider'dan silinen içerikleri fark etmiyor. Gerçek bir provider için tasarım: [Ölçeklenirken ingest](#ölçeklenirken-ingest-sayfalama-ve-artımlı-senkronizasyon).
