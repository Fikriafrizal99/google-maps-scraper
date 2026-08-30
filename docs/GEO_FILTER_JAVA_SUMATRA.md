# Geo Filter Jawa & Sumatra

Branch `feat/java-sumatra-b2b-prospecting` menyediakan pilihan wilayah bertingkat untuk prospecting bisnis publik:

```text
Provinsi
  -> Kabupaten / Kota
      -> Kecamatan
          -> Desa / Kelurahan
```

## Cara kerja

Dashboard tidak menyimpan puluhan ribu desa sebagai file config manual. Pilihan wilayah dimuat secara bertahap dari static API wilayah Indonesia milik `emsifa/api-wilayah-indonesia`:

- `provinces.json`
- `regencies/{provinceId}.json`
- `districts/{regencyId}.json`
- `villages/{districtId}.json`

Sumber default:

```text
https://emsifa.github.io/api-wilayah-indonesia/api
```

Dashboard mem-proxy sumber tersebut melalui endpoint lokal:

```text
GET /api/geo/provinces
GET /api/geo/regencies?province_id=32
GET /api/geo/districts?regency_id=3203
GET /api/geo/villages?district_id=3203010
```

Hanya 16 provinsi di Jawa dan Sumatra yang ditampilkan pada pilihan provinsi.

## Lazy loading dan cache

Data hanya dimuat ketika dibutuhkan:

1. Dashboard dibuka -> daftar provinsi dimuat.
2. Provinsi dipilih -> kabupaten/kota provinsi tersebut dimuat.
3. Kabupaten/kota dipilih -> kecamatan terkait dimuat.
4. Kecamatan dipilih -> desa/kelurahan terkait dimuat.

Setiap respons upstream disimpan ke:

```text
data/geo-cache/
```

Dengan demikian daftar yang sudah pernah dibuka tidak perlu diunduh ulang pada request berikutnya.

Untuk memaksa refresh data wilayah, hapus isi `data/geo-cache/` lalu buka dropdown kembali.

## Scope collector

Pengguna boleh berhenti pada level mana pun.

Contoh provinsi:

```text
JAWA BARAT, Indonesia
```

Contoh kabupaten:

```text
KABUPATEN CIANJUR, JAWA BARAT, Indonesia
```

Contoh kecamatan:

```text
CUGENANG, KABUPATEN CIANJUR, JAWA BARAT, Indonesia
```

Contoh desa:

```text
SUKAMULYA, CUGENANG, KABUPATEN CIANJUR, JAWA BARAT, Indonesia
```

Scope lengkap dikirim ke collector melalui flag:

```text
-location
```

Collector kemudian menggabungkan setiap keyword pada preset dengan satu scope tersebut. Jika preset memiliki 18 keyword dan pengguna memilih satu desa, hanya 18 query yang dibuat untuk desa itu, bukan 288 query lintas provinsi.

## Filter database lead

Lokasi hasil collect disimpan pada kolom `subarea`. Filter lokasi memakai pencocokan hierarkis (`LIKE`), sehingga lead yang dikumpulkan sampai level desa tetap dapat dicari menggunakan scope kecamatan, kabupaten/kota, atau provinsi yang menjadi bagian dari path lokasi tersebut.

## Batasan

Data wilayah adalah data referensi eksternal. Jika terjadi pemekaran/perubahan administrasi dan upstream belum diperbarui, pilihan dashboard juga akan mengikuti data upstream yang tersedia. Cache lokal juga harus dibersihkan untuk mengambil versi upstream yang lebih baru.
