# Customer Excel V1

## Tujuan

Customer Excel adalah format delivery utama untuk database lead yang dijual. CSV tetap dipertahankan sebagai format operasional/fallback, tetapi file `.xlsx` dirancang agar customer menerima produk yang lebih rapi, mudah difilter, dan langsung dapat digunakan.

## Endpoint

```text
/export/customer.xlsx
```

Export mengikuti filter dashboard yang sedang aktif. Lead dengan QC `exclude` tetap otomatis dibuang melalui customer-safe filtering yang sama dengan Customer CSV.

## Dependency

Customer Excel V1 dibuat hanya dengan Go standard library (`archive/zip` + SpreadsheetML/Open XML). Tidak ada dependency Excel eksternal dan tidak ada perubahan `go.mod` / `go.sum`.

## Sheet SUMMARY

Sheet `SUMMARY` berfungsi sebagai cover dan ringkasan paket.

Isi utama:

- judul paket database lead;
- niche;
- wilayah;
- waktu export;
- filter yang digunakan;
- total lead;
- jumlah lead dengan HP;
- jumlah lead dengan website;
- jumlah lead berstatus terverifikasi;
- rata-rata rating;
- catatan penggunaan dan freshness data.

## Sheet LEADS

Sheet `LEADS` berisi data customer-safe dengan kolom:

1. `No`
2. `Nama Kost`
3. `Segment`
4. `Target`
5. `Alamat`
6. `Wilayah`
7. `WhatsApp`
8. `Website`
9. `Rating`
10. `Jumlah Review`
11. `Kisaran Harga`
12. `Fasilitas`
13. `Tipe Sewa`
14. `Furnish`
15. `Aturan`
16. `Landmark`
17. `Selling Point`
18. `Status Verifikasi`
19. `Google Maps`
20. `Foto`

## UX workbook

Customer Excel V1 menggunakan:

- header visual yang konsisten dengan dashboard;
- freeze pane pada header data;
- autofilter pada seluruh tabel lead;
- lebar kolom yang disesuaikan dengan tipe data;
- text wrapping untuk alamat/fasilitas/aturan;
- hyperlink klik langsung untuk WhatsApp, website, Google Maps, dan foto;
- penanda visual status verifikasi;
- gridline disembunyikan agar tampilan lebih seperti report daripada raw spreadsheet.

## Keamanan data

Customer Excel tidak membawa field internal seperti:

- `place_id`;
- `data_id`;
- `source_key`;
- raw JSON `images`;
- catatan QC internal;
- enrichment source/timestamp internal;
- `first_seen` / `last_checked`.

Customer tidak mendapat akses ke master database atau dashboard internal.

## Filter contract

Klik tombol `Export Customer Excel` dari dashboard akan membawa query filter aktif dari halaman dashboard ke endpoint XLSX. Karena itu jumlah lead pada Excel harus sama dengan subset customer-safe pada filter dashboard, kecuali lead dengan QC `exclude` yang selalu dibuang.

Contoh:

```text
segment=putri
verification_status=verified
```

Jika dashboard menampilkan satu lead customer-safe, sheet `LEADS` juga harus berisi satu lead.

## Testing

Jalankan:

```bash
go test ./cmd/leaddashboard ./internal/leadstore
```

Test Customer Excel mencakup:

- paket XLSX dapat dibuka sebagai ZIP Open XML;
- workbook memiliki sheet `SUMMARY` dan `LEADS`;
- filter paket muncul di summary;
- data lead muncul di sheet LEADS;
- autofilter tersedia;
- hyperlink WhatsApp dibuat;
- URL non-HTTP/HTTPS ditolak untuk hyperlink eksternal.

## Commit checkpoints

Fitur dibuat dalam commit kecil agar mudah direvert:

1. `feat: add professional customer xlsx generator`
2. `test: validate customer xlsx workbook structure`
3. `feat: expose customer xlsx export route`
4. `ui: make customer excel the primary delivery export`
5. `docs: document customer excel v1`

Rollback satu tahap:

```bash
git revert <commit-sha>
```

Hindari `git reset --hard` pada branch kerja bersama.

## Validasi operasional

Setelah pull dan build dashboard terbaru:

1. gunakan filter yang menghasilkan subset kecil (contoh `Putri + Terverifikasi`);
2. klik `Export Customer Excel`;
3. buka file `.xlsx`;
4. pastikan `SUMMARY` menampilkan filter yang sama;
5. pastikan jumlah baris pada `LEADS` sesuai hasil filter customer-safe;
6. coba klik link WA dan Google Maps pada satu baris.

Customer CSV tetap tersedia untuk kebutuhan kompatibilitas, tetapi Excel menjadi format delivery utama V1.
