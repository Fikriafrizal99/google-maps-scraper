# Enrichment & Verification Queue V1

## Tujuan

Queue mempercepat pekerjaan manual enrichment dan verifikasi tanpa harus bolak-balik Dashboard → Detail → Save → Dashboard.

Workflow utama:

```text
Filter subset → Review Queue → cek sumber → isi data → Save & Next
```

Keputusan tetap dibuat manusia. Sistem tidak menebak data yang tidak tersedia secara eksplisit.

## Endpoint

```text
GET  /queue
POST /queue/{id}
```

Dashboard memiliki tombol `Review Queue`. Tombol membawa filter dashboard yang sedang aktif ke queue.

## Scope default

Jika tidak ada filter `verification_status`, queue menggunakan mode pending:

- `unverified` masuk queue;
- `needs_check` masuk queue;
- `verified` dilewati;
- QC `exclude` selalu dilewati.

Jika dashboard secara eksplisit difilter `verification_status`, queue mengikuti status tersebut.

Filter lain tetap dibawa:

- pencarian;
- preset/niche;
- area/subarea;
- minimum rating;
- Ada HP;
- segment;
- target;
- QC status.

Karena itu pekerjaan dapat dilakukan per paket, misalnya hanya `Kost Putri Jakarta Selatan` daripada seluruh database sekaligus.

## Tampilan satu lead

Panel kiri menyediakan konteks dan sumber:

- nama/category;
- rating dan jumlah review;
- alamat;
- Google Maps;
- website;
- WhatsApp;
- hingga 4 foto;
- status enrichment dan QC saat ini.

Panel kanan menyediakan form cepat:

- segment;
- target penghuni;
- kisaran harga;
- tipe sewa;
- fasilitas;
- furnish;
- landmark;
- aturan;
- selling point;
- status verifikasi;
- QC lead;
- catatan QC internal.

## Quick actions

- `Exclude & Next`: set QC menjadi `exclude`, simpan data form, lanjut.
- `Perlu Dicek & Next`: set verification menjadi `needs_check`, simpan, lanjut.
- `Simpan & Next`: simpan nilai sesuai form, lanjut.
- `Terverifikasi & Next`: set verification menjadi `verified`, simpan, lanjut.
- `Lewati`: tidak mengubah data dan pindah ke lead berikutnya.

Manual enrichment memakai `UpdateEnrichment`, sehingga source berubah menjadi `manual` dan auto-enrichment berikutnya tidak menimpa keputusan manual.

## Safety

- redirect setelah POST hanya menerima URL internal `/queue`;
- status QC divalidasi sebelum write;
- validasi panjang field enrichment lama tetap digunakan;
- customer export tidak membawa catatan QC internal;
- lead `exclude` tidak ikut customer-safe export.

## Testing

Jalankan:

```bash
go test ./cmd/leaddashboard ./internal/leadstore
```

Regression test queue mencakup:

- pending queue melewati `verified`;
- pending queue melewati `exclude`;
- explicit verified queue tetap dapat digunakan;
- Next URL mempertahankan filter;
- redirect eksternal ditolak.

## Commit checkpoints

1. `feat: add enrichment verification queue ui`
2. `feat: add enrichment verification queue workflow`
3. `feat: expose enrichment verification queue routes`
4. `test: cover enrichment verification queue behavior`
5. `ui: add review queue entry to lead dashboard`
6. `docs: document enrichment verification queue v1`

Rollback satu perubahan menggunakan:

```bash
git revert <commit-sha>
```

Jangan gunakan `git reset --hard` pada branch kerja bersama kecuali memahami dampaknya.

## Validasi operasional

Sesudah pull, test, build, dan restart dashboard:

1. buka Dashboard;
2. opsional: filter subset yang ingin dikerjakan;
3. klik `Review Queue`;
4. pastikan satu lead tampil dengan tombol Maps/Website/WA;
5. isi satu atau dua field;
6. klik `Terverifikasi & Next` atau `Simpan & Next`;
7. pastikan lead berikutnya langsung tampil;
8. kembali ke Dashboard dan cek perubahan tersimpan.
