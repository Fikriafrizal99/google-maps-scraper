# Customer Export V1

## Tujuan

Menyediakan file CSV yang aman dibagikan ke customer tanpa membocorkan master database, identifier internal, catatan QC, atau metadata scraper yang tidak diperlukan customer.

## Endpoint

```text
/export/customer.csv
```

Endpoint mengikuti filter dashboard yang sedang aktif, termasuk:

- pencarian (`q`)
- preset/niche
- area/subarea
- minimum rating
- hanya lead dengan HP
- segment kost
- target penghuni
- status verifikasi
- status QC jika filter tersebut dipilih

## Aturan keamanan data

Lead dengan status QC `exclude` selalu dibuang dari Customer Export, walaupun filter lain cocok.

Field internal berikut tidak boleh masuk Customer Export:

- `place_id`
- `data_id`
- `source_key`
- raw `images`
- `review_status`
- `review_note`
- `reviewed_at`
- `first_seen`
- `last_checked`
- `enrichment_source`
- enrichment internal timestamp

## Field Customer Export V1

1. `name`
2. `category`
3. `address`
4. `area`
5. `subarea`
6. `phone`
7. `website`
8. `rating`
9. `review_count`
10. `maps_url`
11. `photo_url`
12. `segment`
13. `target`
14. `rental_type`
15. `price_range`
16. `facilities`
17. `furnish`
18. `rules`
19. `landmark`
20. `selling_point`
21. `verification_status`

Nilai enrichment `unknown` dikirim sebagai kolom kosong agar file customer lebih bersih.

## Format

V1 menggunakan CSV UTF-8 dengan BOM agar lebih nyaman dibuka langsung di Microsoft Excel pada Windows.

## Internal vs Customer Export

Dashboard menyediakan dua export berbeda:

- **Export Internal CSV**: untuk operasional internal dan audit; membawa metadata QC/enrichment lebih lengkap.
- **Export Customer CSV**: hanya field customer-safe dan otomatis membuang lead `Exclude`.

Customer tidak mendapat akses ke dashboard atau master database.

## Testing

Jalankan:

```bash
go test ./cmd/leaddashboard ./internal/leadstore
```

Test customer export memastikan:

- lead `Exclude` tidak ikut;
- field internal terlarang tidak ada di header;
- nilai `unknown` tidak dikirim sebagai teks literal.

## Rollback

Fitur ini dibuat dalam commit terpisah. Jika perlu membatalkan satu perubahan, gunakan:

```bash
git revert <commit-sha>
```

Jangan menggunakan `git reset --hard` pada branch kerja bersama kecuali memahami dampaknya.

## Tahap setelah V1

Setelah CSV tervalidasi secara operasional, opsi berikutnya adalah Excel customer delivery yang lebih profesional dengan dua sheet:

- `SUMMARY`
- `LEADS`

Excel belum ditambahkan pada V1 agar alur bisnis dan isi customer export divalidasi terlebih dahulu.
