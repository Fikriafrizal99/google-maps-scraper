# KOST UI Reference — 1:1 Implementation Target

Folder ini adalah **source of truth untuk UI/UX KOST Lead Control**.

Semua pengembangan tampilan berikutnya harus mengacu ke gambar pada folder ini dan ditujukan untuk menghasilkan tampilan yang **semirip mungkin 1:1** pada ukuran desktop referensi, tanpa membuat redesign baru atau mengubah hierarchy visual tanpa persetujuan.

> **Penting:** data/nama/nilai yang terlihat pada mockup hanyalah contoh visual. Implementasi tetap harus memakai data asli dari aplikasi, database, filter, enrichment, review, dan hasil collector yang sudah ada.

## Reference files

### 1. Dashboard / Lead Database

**File:** [`Lead Control Dashboard.png`](./Lead%20Control%20Dashboard.png)

![Dashboard UI Reference](./Lead%20Control%20Dashboard.png)

Halaman target:

- route utama `/`
- statistik ringkas
- Collect / Refresh Data
- filter lokasi Provinsi → Kabupaten/Kota → Kecamatan → Desa/Kelurahan
- filter segment, target, verifikasi, QC, nomor HP
- tabel leads
- pagination
- Review Queue
- export internal/customer

Implementasi harus mengikuti reference untuk:

- header dan hierarchy halaman
- ukuran dan weight judul
- ukuran body text dan label
- card statistics
- jarak antar section
- radius card/input/button
- posisi tombol
- alignment form
- tabel, badge, pagination, dan action
- warna status dan states

---

### 2. Preview / Detail Kost

**File:** [`Lead Control.png`](./Lead%20Control.png)

![Detail Kost UI Reference](./Lead%20Control.png)

Halaman target:

- route `/lead/{id}`
- gallery/foto kost
- informasi utama kost
- Google Maps / WhatsApp / Website
- Keterangan Kost / enrichment
- Quality Control
- informasi metadata

### Navigasi wajib

Detail kost **tidak boleh memaksa user kembali ke dashboard untuk membuka kost berikutnya**.

Harus tersedia navigasi:

```text
← Sebelumnya        X dari Y hasil        Selanjutnya →
```

Navigasi ditampilkan **di bagian atas dan bawah halaman detail** seperti reference.

Aturan navigasi:

- `Sebelumnya` membuka lead sebelum lead aktif.
- `Selanjutnya` membuka lead setelah lead aktif.
- urutan mengikuti daftar/filter tempat user membuka detail.
- filter aktif harus dipertahankan.
- posisi/pagination asal harus dapat dipertahankan untuk tombol kembali ke database.
- tombol disabled bila tidak ada item sebelumnya/selanjutnya.
- nama lead sebelumnya/selanjutnya boleh ditampilkan sebagai konteks seperti reference.

Navigasi **tidak boleh hanya berdasarkan ID database**, karena urutan ID belum tentu sama dengan hasil filter yang sedang ditampilkan.

---

### 3. Review Queue

**File:** [`Review Queue.png`](./Review%20Queue.png)

![Review Queue UI Reference](./Review%20Queue.png)

Halaman target:

- route `/queue`
- preview lead yang sedang direview
- enrichment/edit field
- status verifikasi
- Quality Control
- catatan internal
- progress review
- previous / next queue item
- save & next workflow

Tujuan UX halaman ini adalah user dapat memproses banyak lead secara berurutan tanpa bolak-balik ke dashboard.

---

## Design rules

### Typography

Typography harus konsisten di seluruh halaman.

Target hierarchy:

| Element | Target |
|---|---|
| App / page title | paling dominan, weight bold |
| Section title | konsisten antar card/section |
| Lead title | lebih kuat dari metadata |
| Label form/table | medium / semibold |
| Body text | regular |
| Helper / metadata | ukuran lebih kecil, warna muted |

Jangan menggunakan ukuran heading yang berbeda-beda untuk level yang sama.

Font baseline implementasi adalah **Inter / system sans-serif** agar sesuai dengan karakter visual reference dan UI yang sudah ada.

### Spacing

Gunakan spacing yang konsisten. Hindari margin/padding manual yang berbeda untuk komponen sejenis.

Target visual:

- section mempunyai jarak vertikal yang jelas
- card mempunyai padding konsisten
- form field mempunyai tinggi konsisten
- button mempunyai tinggi dan radius konsisten
- grid alignment harus rapi

### Colors

Gunakan warna reference sebagai acuan:

- primary action: blue
- positive / verified / valid: green
- warning / needs check: amber/orange
- destructive / exclude: red
- neutral metadata: gray/slate
- page background: very light gray/blue
- card: white

Jangan menambah warna baru jika tidak diperlukan oleh status atau aksi yang sudah ada.

### Components

Komponen yang sama harus terlihat sama di semua halaman:

- primary button
- secondary button
- inputs
- selects
- textarea
- badges
- cards
- section headers
- pagination/navigation
- table header/body

Tidak boleh ada satu halaman memakai style lama sementara halaman lain memakai style reference baru.

---

## Functional constraints

Redesign UI **tidak boleh merusak fungsi yang sudah ada**.

Harus tetap bekerja:

- hierarchical geo filter
- collector
- pencarian/filter dashboard
- pagination
- lead detail
- enrichment save
- review/QC save
- Review Queue
- export CSV/PDF/XLSX
- Google Maps
- WhatsApp
- Website
- authentication `LEADS_USER` / `LEADS_PASS`

Visual boleh dirombak untuk mengikuti reference, tetapi backend behavior dan existing data contract harus dipertahankan kecuali perubahan memang dibutuhkan untuk fitur navigasi detail.

---

## Responsive behavior

Desktop reference adalah target visual utama.

Untuk viewport lebih kecil:

- hierarchy visual tetap sama
- tidak boleh menyebabkan horizontal overflow pada halaman utama
- form/grid boleh collapse menjadi lebih sedikit kolom
- action penting harus tetap mudah diakses
- previous/next detail harus tetap terlihat dan usable
- tabel boleh memakai horizontal scroll bila memang diperlukan

Responsive implementation tidak diwajibkan pixel-identical terhadap desktop, tetapi harus mempertahankan design language yang sama.

---

## Acceptance criteria

Sebelum UI dianggap selesai, lakukan perbandingan side-by-side antara aplikasi dan reference.

Checklist minimum:

- [ ] Dashboard mengikuti `Lead Control Dashboard.png`
- [ ] Detail mengikuti `Lead Control.png`
- [ ] Review Queue mengikuti `Review Queue.png`
- [ ] typography seragam
- [ ] heading hierarchy seragam
- [ ] spacing dan radius seragam
- [ ] button/input/select seragam
- [ ] badge/status seragam
- [ ] previous/next pada Detail bekerja sesuai hasil/filter aktif
- [ ] previous/next tersedia di atas dan bawah Detail
- [ ] Review Queue dapat diproses berurutan tanpa kembali ke dashboard
- [ ] seluruh fungsi lama tetap berjalan
- [ ] desktop visual sudah dibandingkan side-by-side dengan reference

## Change policy

File reference di folder ini **tidak boleh dianggap sekadar inspirasi**.

Reference adalah target implementasi. Jika akan mengubah layout, hierarchy, typography, warna utama, atau pola navigasi dari gambar reference, perubahan tersebut harus disepakati terlebih dahulu.
