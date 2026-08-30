# Kost Enrichment V1

## Tujuan

Menambah data keterangan kost di atas master lead Google Maps tanpa mengubah data mentah hasil scraper. Enrichment dipakai untuk filtering internal dan nantinya customer export.

## Prinsip

1. Jangan menebak data yang tidak tersedia. Nilai yang belum diketahui disimpan sebagai `unknown`.
2. Data enrichment disimpan terpisah dari tabel `leads`, sehingga refresh scraper tidak menghapus keterangan.
3. Auto-enrichment hanya membaca sinyal eksplisit dari data yang sudah ada (terutama title, category, address).
4. Keputusan manual selalu menang. Row dengan `source=manual` tidak boleh ditimpa auto-enrichment berikutnya.
5. Category Google Maps bukan sumber kebenaran tunggal. Salah kategori tidak mengubah segmentasi kost secara otomatis.

## Field V1

| Field | Contoh | Catatan |
| --- | --- | --- |
| `segment` | `putri`, `putra`, `campur`, `pasutri` | `unknown` jika tidak eksplisit |
| `target` | `mahasiswa`, `karyawan`, `keluarga` | Tidak menganggap `umum` tanpa bukti |
| `rental_type` | `harian`, `mingguan`, `bulanan`, `tahunan` | Bisa lebih dari satu, dipisahkan koma |
| `price_range` | `unknown` | V1 tidak menebak harga |
| `facilities` | `AC, WiFi, Kamar mandi dalam` | Hanya sinyal eksplisit |
| `furnish` | `furnished`, `semi furnished`, `full furnished` | Hanya jika tertulis |
| `rules` | `boleh pasutri`, `pet friendly`, `parkir mobil` | Hanya jika tertulis |
| `landmark` | `unknown` | Belum diekstrak otomatis di V1 |
| `selling_point` | `Eksklusif`, `Premium`, `Pet friendly` | Hanya dari wording eksplisit |
| `verification_status` | `unverified`, `needs_check`, `verified` | Default `unverified` |
| `source` | `auto`, `manual` | Manual tidak ditimpa auto |
| `updated_at` | RFC3339 UTC | Waktu enrichment terakhir |

## Storage

Enrichment disimpan di tabel SQLite `lead_enrichment` dengan `lead_id` sebagai primary key. Tabel ini terpisah dari `leads` agar upsert collector tidak menyentuh data enrichment.

## Auto-enrichment V1

Auto-enrichment saat ini dapat mengenali antara lain:

- Segmentasi: putri, putra, campur, pasutri.
- Target: mahasiswa, karyawan, keluarga.
- Tipe sewa: harian, mingguan, bulanan, tahunan.
- Fasilitas eksplisit: WiFi, AC, kamar mandi dalam, parkir, dapur, laundry.
- Furnish eksplisit.
- Aturan eksplisit seperti pet friendly dan boleh pasutri.
- Selling point sederhana seperti eksklusif dan premium.

Jika tidak ada sinyal, field tetap `unknown`.

## Preview dan Apply

Build CLI:

```bash
go build -o bin/leadenrich ./cmd/leadenrich
```

Preview tanpa menulis database:

```bash
./bin/leadenrich \
  -preset kost \
  -area jakarta \
  -subarea "Jakarta Selatan"
```

Apply hanya setelah preview dinilai masuk akal:

```bash
./bin/leadenrich \
  -preset kost \
  -area jakarta \
  -subarea "Jakarta Selatan" \
  -apply
```

## Testing

Jalankan:

```bash
go test ./internal/leadstore ./cmd/leadenrich
```

Test V1 mencakup:

- deteksi putri/putra/pasutri/campur;
- data ambigu tetap `unknown`;
- deteksi fasilitas dan tipe sewa;
- manual enrichment tidak ditimpa auto-enrichment;
- default enrichment untuk lead yang belum pernah diproses.

## Commit checkpoints

Fitur V1 sengaja dipecah ke commit kecil:

1. `feat: add kost enrichment storage`
2. `feat: add conservative kost auto enrichment`
3. `test: cover kost enrichment rules and manual override`
4. `feat: add kost enrichment preview cli`
5. `docs: document kost enrichment v1`

Untuk membatalkan satu perubahan tanpa membuang commit setelahnya, gunakan:

```bash
git revert <commit-sha>
```

Jangan gunakan `git reset --hard` pada branch bersama kecuali benar-benar memahami dampaknya.

## Tahap berikutnya

Setelah test dan preview lokal lolos:

1. tampilkan enrichment di detail lead dashboard;
2. tambah form manual edit;
3. tambah filter segment/target/verification;
4. baru integrasikan ke customer-safe export.
