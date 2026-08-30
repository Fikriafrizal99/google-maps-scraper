# Customer Delivery V2

## Tujuan

Customer Delivery V2 memisahkan dua kebutuhan customer:

- **PDF Catalog** untuk melihat lead secara visual dan nyaman;
- **Excel Database** untuk filter, sort, dan pengolahan data.

Dashboard hanya menampilkan satu menu `Export Customer` dengan dua pilihan tersebut. Endpoint Customer CSV tetap dipertahankan sebagai fallback, tetapi tidak lagi menjadi aksi utama di toolbar.

## Filter dan keamanan

PDF dan Excel memakai filter dashboard yang sedang aktif dan pipeline `customerSafeRows` yang sama dengan Customer CSV.

Artinya:

- filter niche/area/subarea/rating/HP/segment/target/verifikasi/QC tetap dibawa ke export;
- lead dengan QC `exclude` tetap dibuang dari customer delivery;
- field internal seperti `place_id`, `data_id`, `source_key`, raw images JSON, catatan QC internal, dan timestamp enrichment internal tidak diekspos.

URL export dibangun penuh dari Go. Query filter tidak lagi ditempel sebagai fragmen HTML/JavaScript sehingga tidak mengulang regresi export yang sebelumnya kehilangan filter.

## PDF Catalog

Endpoint:

```text
/export/customer.pdf
```

PDF adalah format visual utama untuk customer.

### Struktur

- halaman cover;
- judul paket otomatis, misalnya `Database Kost Putri Jakarta Selatan`;
- KPI total lead, coverage HP, verified, dan rata-rata rating;
- scope/filter paket;
- freshness data;
- satu halaman katalog per lead;
- foto langsung tertanam di PDF;
- harga, tipe sewa, rating, target, alamat, fasilitas, dan informasi tambahan;
- tombol/link klik untuk WhatsApp, Website, dan Google Maps;
- timestamp freshness per lead tanpa mengekspos metadata internal lain.

### Jumlah foto

Untuk menjaga ukuran file dan waktu export, jumlah foto per lead menyesuaikan ukuran paket:

| Jumlah lead | Maksimum foto / lead |
| --- | ---: |
| 1–60 | 3 |
| 61–150 | 2 |
| >150 | 1 |

Jika sebuah foto gagal diambil atau format tidak dapat diproses, PDF tetap dibuat dan slot foto menampilkan placeholder.

Foto diunduh hanya dari URL `http/https`, dinormalisasi menjadi JPEG, dan diperkecil maksimum 1000 px agar file tidak terlalu berat.

## Excel Database V1.2

Endpoint:

```text
/export/customer.xlsx
```

Excel V1.2 dibuat lebih data-first karena fungsi visual sudah ditangani PDF.

### Sheet SUMMARY

Tetap menggunakan ringkasan V1.1:

- judul paket;
- niche dan wilayah;
- total lead;
- coverage HP;
- coverage segment;
- coverage fasilitas;
- coverage website;
- coverage harga;
- verified;
- rata-rata rating;
- freshness;
- filter yang digunakan.

### Sheet LEADS

Kolom:

1. No
2. Nama Kost
3. Segment
4. Target
5. Alamat
6. Wilayah
7. WhatsApp
8. Website
9. Rating
10. Jumlah Review
11. Kisaran Harga
12. Fasilitas
13. Tipe Sewa
14. Furnish
15. Aturan
16. Landmark
17. Selling Point
18. Status Verifikasi
19. Google Maps

Kolom `Foto / Lihat Foto` dihilangkan dari Excel V1.2. Customer diarahkan ke PDF Catalog untuk pengalaman visual.

Excel tetap memiliki:

- freeze header;
- autofilter;
- hyperlink WA/Website/Maps;
- auto-hide untuk kolom optional yang 100% kosong;
- customer-safe fields saja.

## Dashboard UI

Toolbar customer sekarang menggunakan satu dropdown:

```text
Export Customer ▾
├── PDF Catalog
└── Excel Database
```

PDF ditempatkan sebagai pilihan pertama karena merupakan format presentasi/customer-facing. Excel menjadi companion database untuk analisis lanjutan.

## Dependency

Customer PDF dan Excel V1.2 tidak menambah dependency Go baru.

PDF dibuat dengan Go standard library dan writer PDF internal. Image normalization menggunakan package `image`, `image/jpeg`, `image/png`, dan `image/gif` standard library.

## Testing

Jalankan:

```bash
go test ./cmd/leaddashboard ./internal/leadstore
go build -o bin/leaddashboard ./cmd/leaddashboard
```

Regression test mencakup:

- PDF memiliki header PDF yang valid;
- PDF membawa image XObject dan link annotations;
- package title dan data lead muncul;
- jumlah foto adaptif sesuai ukuran paket;
- Excel V1.2 menggunakan 19 kolom A:S;
- Excel tidak lagi memiliki kolom/link foto;
- dropdown PDF dan Excel membawa filter aktif;
- URL customer delivery menolak base URL eksternal.

## Validasi operasional

Setelah pull, test, build, dan restart dashboard:

1. aktifkan filter customer, misalnya `Putri + Terverifikasi`;
2. buka `Export Customer`;
3. pilih `PDF Catalog` dan pastikan jumlah lead sesuai filter;
4. cek foto tampil langsung di halaman lead PDF;
5. cek link WhatsApp, Website, dan Maps;
6. pilih `Excel Database` dan pastikan sheet `LEADS` memiliki subset lead yang sama;
7. pastikan Excel tidak lagi memerlukan link foto untuk pengalaman visual.

Untuk paket besar, PDF tetap dapat diekspor tetapi jumlah foto per lead otomatis dikurangi. Paket terkurasi 30–60 lead memberikan pengalaman katalog visual paling lengkap.

## Commit checkpoints

Implementasi dipisahkan menjadi commit kecil agar mudah direvert:

- customer PDF core;
- PDF layout;
- image pipeline;
- PDF drawing helpers;
- PDF writer;
- PDF regression test;
- Excel V1.2 data-first;
- Excel V1.2 regression test;
- safe customer delivery URLs;
- PDF/XLSX route activation;
- dropdown UI;
- delivery URL regression test;
- dokumentasi Customer Delivery V2.

Rollback dilakukan dengan `git revert <commit-sha>`. Hindari `git reset --hard` pada branch kerja bersama.
