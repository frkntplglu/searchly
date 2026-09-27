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
| `api` | OpenAPI spesifikasyonu ve Swagger UI sayfası (binary'ye gömülü) |
| `internal/dashboard` | Binary'ye gömülü web arayüzü (HTML + vanilla JS), `/` altında servis edilir |
| `internal/ingest` | Provider'lardan veriyi çekip kaydeden ingest mantığı (tek seferlik ya da periyodik) |
| `internal/database` | pgx bağlantı havuzu ve gömülü SQL migration'ları |
| `api` | OpenAPI / proto tanımları |

## Hızlı başlangıç

```sh
docker compose up -d --build
```

Bu komut sırasıyla şunları yapar: PostgreSQL'i başlatır, şemayı `db.sql`'den oluşturur (şema zaten varsa atlar), ingest worker'ı (5 dakikada bir) ve API'yi başlatır. Ardından:

- Dashboard: `http://localhost:8080`
- API dokümantasyonu: `http://localhost:8080/docs`

Ayarlar proje kökündeki `.env` dosyasından okunur. Dosya yoksa varsayılan değerler kullanılır.

## Komutlar

```sh
make db-up    # Postgres'i başlat (localhost:5432)
make migrate  # şemayı db.sql'den oluştur (boş DB'de bir kez çalıştırılır)
make ingest   # provider'lardan veriyi bir kez çek ve kaydet
make ingest-worker  # 5 dakikada bir ingest et (Ctrl+C ile durur)
make run    # sunucuyu başlat (varsayılan port 8080)
make build  # bin/searchly üret
make test   # testleri çalıştır
make lint   # go vet
```

Dashboard: `http://localhost:8080`

API dokümantasyonu: `http://localhost:8080/docs` (Swagger UI). Spesifikasyon `api/openapi.yaml` dosyasında, `/openapi.yaml` adresinden de indirilebilir.

Spesifikasyon elle yazılıyor ve bir sözleşme testiyle (`internal/server/openapi_test.go`) koda bağlı tutuluyor: test API'ye gerçek istekler atıp hem istekleri hem yanıtları spesifikasyona karşı doğruluyor, spesifikasyondaki her endpoint'in test edildiğini de kontrol ediyor. Doküman koddan koparsa `make test` kırılır.

Sağlık kontrolü: `curl localhost:8080/health`

Şema `db.sql` dosyasında. `make migrate` bu dosyayı okuyup veritabanında çalıştırır. İlk kurulumda bir kez çalıştırılması yeterli.

## Tasarım kararları

### Yayın tarihleri gün hassasiyetinde saklanır

Provider'lar yayın tarihini farklı hassasiyette veriyor:

| Provider | Örnek | Hassasiyet |
|---|---|---|
| provider1 (JSON) | `2024-03-15T10:00:00Z` | saniye |
| provider2 (XML) | `2024-03-15` | gün |

Güncellik puanı yayın tarihine göre hesaplanıyor (1 hafta içinde +5, 1 ay içinde +3, 3 ay içinde +1). Tarihler olduğu gibi saklansaydı, provider2'nin içerikleri her zaman günün başında (00:00) yayınlanmış sayılırdı. Aynı gün yayınlanan iki içerikten provider2'deki, provider1'deki içerikten saatler önce yayınlanmış gibi görünürdü. Bu da eşik sınırlarında (örneğin tam 7. gün) aynı günün içeriğinin farklı puan almasına yol açabilirdi.

İki provider'a eşit davranmak için provider1'in zaman damgası UTC'ye çevrilip gün kısmına indiriliyor. Böylece her iki provider'ın tarihleri de `YYYY-MM-DD 00:00 UTC` olarak saklanıyor. Puanlama gün bazında olduğu için saat bilgisinin kaybı sonucu etkilemiyor.

### Skor: sabit kısım saklanır, güncellik sorgu anında eklenir

Skor formülü iki farklı hızda değişen parçadan oluşuyor:

- **Sabit kısım** `(temel puan × tür katsayısı) + etkileşim puanı` yalnızca metrikler değiştiğinde değişir. Upsert sırasında Go'da (`model.Content.BaseScore`) hesaplanıp `base_score` kolonuna yazılır.
- **Güncellik puanı** veri değişmese de zamanla değişir. Saklansaydı bayatlardı. Bu yüzden arama sorgusunda, içeriğin UTC takvim günü cinsinden yaşına göre SQL'de eklenir (`base_score + CASE ...`). Sıralama da bu ifadeye göre yapılır.

Böylece skor her an doğru olur ve zamanlanmış bir işe ihtiyaç duyulmaz. Bedeli, skora göre sıralamanın bir index'ten yararlanamamasıdır.

**Ölçek büyürse:** Tarihler gün hassasiyetinde ve güncellik eşikleri gün cinsinden olduğu için, güncellik puanı bir UTC günü boyunca sabit kalır. Tam skor bir `score` kolonunda tutulup günde bir kez (ya da her ingest'te) tek bir `UPDATE` ile yenilenebilir ve bu kolon index'lenebilir.

### Arama: full-text search, önek eşleşmesi ve alakalılık

Arama, PostgreSQL full-text search ile yapılıyor. Başlık (ağırlık `A`) ve etiketlerden (ağırlık `B`) iki vektör otomatik üretiliyor, ikisi de GIN index'li:

- `search_vector` (`english`): kelimeler köklerine indirilir, böylece "tips" → "Tips", "pattern" → "Patterns" eşleşir.
- `search_vector_simple` (`simple`): kelimeler olduğu gibi saklanır. Kökünden uzun yarım kelimeler ("concurren" → "concurrency", kökü `concurr`) buradan eşleşir.

Bir içerik iki vektörden birinde eşleşirse sonuçlara girer.

- **Önek eşleşmesi:** Aranan her kelime önek olarak eşleşiyor ("concur" → "concurrency"). Böylece kullanıcı yazarken sonuç görüyor. Birden fazla kelime yazılırsa hepsinin geçmesi gerekiyor.
- **Alakalılık:** `sort=relevance` (kelime verildiğinde varsayılan) sonuçları `ts_rank` ile sıralıyor. Tam kelime eşleşmesi önek eşleşmesinden iki kat değerli sayılıyor, böylece "go" aramasında "Go" geçen içerik "Google" geçenin üstünde çıkıyor. Önek puanı iki vektörden yüksek olanı alınarak hesaplanıyor. Toplanmıyor, çünkü toplansaydı iki vektörde birden eşleşen yarım kelimeler çift puan alırdı. Başlıktaki eşleşme etiketteki eşleşmeden daha değerli. Eşitlikte skor belirliyor.
- **Popülerlik:** `sort=popularity` sonuçları skora göre sıralıyor.

**Bilinçli takas:** PostgreSQL'in `websearch_to_tsquery` fonksiyonu `or`, `-kelime` ve `"tırnaklı ifade"` sözdizimini destekliyor ama önek eşleşmesini desteklemiyor. Yazarken arama, bu ileri seviye sözdiziminden daha değerli görüldü. Sorgu Go'da kuruluyor: girdiden sadece harf ve rakamlar alınıyor, böylece özel karakterler sorguyu bozamıyor. Sorgu her zaman parametre olarak gönderiliyor.

## To be considered

Aramanın bilinen ve bilinçli olarak bırakılmış uç durumları:

- **Kelime içermeyen arama her şeyi döndürür.** Sadece sembollerden oluşan bir arama (`&& !!`) temizlendikten sonra boş kalır ve filtre uygulanmaz. Boş bir aramayla aynı şekilde bütün içerikler listelenir.
- **Stop word'ler sadece yarım kelime olarak eşleşir.** `english` yapılandırması "the", "a" gibi kelimeleri yok sayar. Bu yüzden "the" araması `simple` vektörde "the" ile başlayan kelimeleri ("theory", "them") bulur, "The" kelimesinin kendisini alakalılıkta öne çıkarmaz.
- **Önek eşleşmesi kısa kelimelerde geniş sonuç verir.** "go" araması "Google" gibi "go" ile başlayan kelimeleri de bulur. Tam eşleşmeler alakalılık sıralamasında önde tutulduğu için bunlar listenin altında kalır.
- **`or`, `-kelime` ve `"tırnaklı ifade"` sözdizimi desteklenmez.** Önek eşleşmesi için bilinçli olarak bırakıldı (bkz. "Arama" kararı). Bu karakterler sıradan ayraç olarak yok sayılır.
