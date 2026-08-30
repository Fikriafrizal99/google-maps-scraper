# Customer Excel V1.1

## Tujuan

Customer Excel V1.1 memoles workbook customer agar lebih layak dijual sebagai produk data, bukan sekadar hasil export tabel.

V1.1 tetap memakai endpoint yang sama:

```text
/export/customer.xlsx
```

Filter dashboard dan aturan customer-safe tidak berubah. Lead dengan QC `exclude` tetap otomatis dibuang.

## Perubahan utama

### 1. Judul paket otomatis

Judul workbook mengikuti scope/filter yang tersedia.

Contoh:

```text
Database Kost Putri Jakarta Selatan
```

Urutan judul:

- `Database`
- niche/preset
- segment jika difilter
- target jika difilter
- wilayah/subarea

Jika preset atau wilayah tidak dikirim sebagai query filter, V1.1 mencoba mengambil scope yang konsisten dari data hasil export.

Nama file juga mengikuti judul paket, misalnya:

```text
database-kost-putri-jakarta-selatan-20260830-120000.xlsx
```

### 2. Coverage KPI di SUMMARY

Sheet `SUMMARY` sekarang menampilkan coverage agar customer langsung memahami kualitas dan kelengkapan paket.

KPI V1.1:

- total lead;
- rata-rata rating;
- coverage nomor HP;
- coverage segment;
- coverage fasilitas;
- coverage website;
- coverage harga;
- jumlah/persentase lead terverifikasi.

Coverage ditampilkan sebagai count + persentase, misalnya:

```text
213/280 • 76%
```

### 3. Freshness data

`SUMMARY` menampilkan `Data Terakhir Dicek` berdasarkan nilai `last_checked` terbaru dari lead yang masuk ke paket.

Waktu customer ditampilkan dalam WIB dengan nama bulan Indonesia, misalnya:

```text
30 Agu 2026 10:26 WIB
```

Sheet `LEADS` juga menampilkan freshness paket pada subjudul, tanpa membocorkan timestamp internal per lead.

### 4. Kolom kosong otomatis disembunyikan

Kolom opsional yang 100% kosong pada paket secara otomatis diberi status hidden di Excel.

Kolom yang dapat disembunyikan antara lain:

- Segment
- Target
- WhatsApp
- Website
- Kisaran Harga
- Fasilitas
- Tipe Sewa
- Furnish
- Aturan
- Landmark
- Selling Point
- Google Maps
- Foto

Kolom inti seperti nama lead, alamat, wilayah, rating, jumlah review, dan status verifikasi tetap terlihat.

Contoh: jika satu paket tidak memiliki data harga dan landmark sama sekali, kedua kolom tersebut tidak memenuhi layar customer dengan kolom kosong.

## Compatibility

Generator Customer Excel V1 lama tetap berada di codebase sebagai fallback. Endpoint dashboard sekarang diarahkan ke handler V1.1.

Tidak ada dependency baru dan tidak ada perubahan `go.mod` / `go.sum`.

## Testing

Jalankan:

```bash
go test ./cmd/leaddashboard ./internal/leadstore
```

Regression test V1.1 mencakup:

- judul `Database Kost Putri Jakarta Selatan`;
- KPI coverage;
- freshness WIB;
- kolom enrichment kosong diberi `hidden=1`;
- kolom fasilitas tetap terlihat jika memiliki data;
- autofilter tetap tersedia;
- judul dapat fallback ke scope data;
- nama file mengikuti judul paket.

## Commit checkpoints

1. `feat: polish customer excel v1.1 packaging`
2. `test: cover customer excel v1.1 packaging`
3. `feat: activate customer excel v1.1 export`
4. `docs: document customer excel v1.1`

Rollback aktivasi V1.1 tanpa menghapus generator:

```bash
git revert <commit-activate-v1.1>
```

Rollback fitur V1.1 secara penuh dapat dilakukan dengan `git revert` commit terkait secara berurutan. Hindari `git reset --hard` pada branch kerja bersama.

## Validasi operasional

Sesudah pull/build/restart dashboard:

1. pilih filter customer, misalnya `Putri + Terverifikasi`;
2. klik `Export Customer Excel`;
3. buka sheet `SUMMARY` dan cek judul, coverage, dan freshness;
4. buka sheet `LEADS` dan pastikan jumlah lead sesuai filter;
5. pastikan kolom yang benar-benar kosong tidak terlihat;
6. pastikan WhatsApp, Website, Maps, dan Foto tetap dapat diklik jika datanya tersedia.
