# B2B Prospecting — Jawa & Sumatra

Branch: `feat/java-sumatra-b2b-prospecting`

## Tujuan

Menggunakan Google Maps scraper sebagai sumber **prospek bisnis publik** di Jawa dan Sumatra. Sistem ini tidak menginferensikan kondisi finansial individu dan tidak mengumpulkan data pribadi non-publik. Kontak yang dipakai adalah kanal bisnis yang memang dipublikasikan oleh usaha tersebut.

## Coverage

Area `java-sumatra` mencakup 16 provinsi:

### Jawa
- DKI Jakarta
- Banten
- Jawa Barat
- Jawa Tengah
- DI Yogyakarta
- Jawa Timur

### Sumatra
- Aceh
- Sumatera Utara
- Sumatera Barat
- Riau
- Kepulauan Riau
- Jambi
- Bengkulu
- Sumatera Selatan
- Kepulauan Bangka Belitung
- Lampung

## Preset awal

Preset `b2b-prospecting` menggunakan 18 kategori usaha:

- bengkel mobil
- bengkel motor
- toko sparepart mobil
- toko sparepart motor
- toko ban
- toko bangunan
- material bangunan
- kontraktor
- distributor
- supplier
- grosir sembako
- toko sembako
- percetakan
- konveksi
- laundry
- catering
- mebel
- toko furniture

Dengan 18 keyword x 16 provinsi, collector menghasilkan **288 query** sebelum filtering dan deduplication.

## Filter

Lead harus memiliki:

- nama usaha
- alamat
- telepon bisnis publik

Nama usaha yang menunjukkan bahwa entitas tersebut merupakan penyedia pembiayaan/kompetitor dikeluarkan, misalnya `bank`, `leasing`, `finance`, `pinjaman`, `pegadaian`, dan `koperasi simpan pinjam`.

## Build

```bash
make build
```

Binary scraper utama akan dibuat sebagai:

```text
bin/google_maps_scraper
```

## Jalankan seluruh Jawa + Sumatra

```bash
go run ./cmd/collector \
  -preset b2b-prospecting \
  -area java-sumatra \
  -output data/b2b-java-sumatra.csv \
  -db data/b2b-leads.db
```

## Jalankan satu provinsi untuk smoke test

Contoh Jawa Barat:

```bash
go run ./cmd/collector \
  -preset b2b-prospecting \
  -area java-sumatra \
  -subarea "Jawa Barat" \
  -output data/b2b-jawa-barat.csv \
  -db data/b2b-leads.db
```

Contoh Sumatera Utara:

```bash
go run ./cmd/collector \
  -preset b2b-prospecting \
  -area java-sumatra \
  -subarea "Sumatera Utara" \
  -output data/b2b-sumatera-utara.csv \
  -db data/b2b-leads.db
```

## Output

Field awal:

- place_id
- data_id
- title
- category
- address
- phone
- website
- latitude
- longitude
- rating
- reviews
- link

Deduplication menggunakan prioritas:

1. place_id
2. data_id
3. phone
4. title + coordinates

## Cara memprioritaskan prospek

Jangan membuat skor berdasarkan dugaan bahwa sebuah bisnis sedang kesulitan keuangan. Gunakan **business-fit/contactability score**, misalnya:

- kategori usaha sesuai target
- telepon bisnis tersedia
- alamat lengkap
- listing aktif dan memiliki review
- bukan lembaga pembiayaan/kompetitor
- belum pernah dihubungi

Kebutuhan pembiayaan dikonfirmasi hanya setelah pemilik/pengelola usaha memberikan respons atau mengisi jalur opt-in.

## Catatan coverage

Area saat ini membagi pencarian per provinsi. Ini cocok untuk MVP dan smoke test, tetapi Google Maps dapat membatasi jumlah hasil untuk query area yang terlalu luas. Jika hasil valid, fase berikutnya adalah menambah area per `kabupaten/kota` agar coverage Jawa dan Sumatra lebih merata tanpa mengubah scraper core.

## Peran SearXNG dan twscrape

- **SearXNG:** opsional untuk riset kategori, direktori usaha, dan halaman publik; tidak perlu menjadi core collector.
- **twscrape:** tidak dipakai sebagai baseline. Ia membutuhkan akun X/Twitter terautorisasi dan mekanisme account pool, sehingga lebih rapuh untuk workflow produksi dan mudah bergeser ke profiling individu.
- **Google Maps collector:** tetap menjadi core untuk prospecting bisnis publik karena source dan output-nya paling sesuai dengan kebutuhan awal.
