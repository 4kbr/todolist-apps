# Task 06 — Domain dan Usecase Modul Task

**Phase:** 4 — Task
**Bergantung pada:** task 05
**Status:** belum dikerjakan

## Tujuan

Setelah task ini selesai, modul `task` punya `domain` dan `application` yang
lengkap dan teruji — `List` sebagai agregat root berisi `Todo`, semua usecase
CRUD plus pindah-list dan selesai/batal-selesai, dan fake repository tulis
tangan yang membuktikan aturan bisnisnya benar. Tidak ada satu baris SQL atau
HTTP di task ini — itu task 07. Kalau kompilasi butuh `net/http`, `pgx`, atau
`chi`, sesuatu salah tempat.

## Rujukan

- ADR-003 (modul lain tidak di-import — modul `task` tidak boleh mengimpor
  `identity` sama sekali)
- ADR-004 (List agregat root, Todo di dalamnya, satu modul)
- ADR-009 (UUIDv7 — dipakai lewat `internal/platform/id`, diterima usecase
  sebagai parameter, bukan dibuat di domain)
- ADR-011 (`todos.user_id` didenormalisasi — ini alasan `MoveTo` wajib
  memperbarui `Todo.UserID`)
- `ARCHITECTURE.md` bagian "Arah dependency", "Komunikasi antar modul", tabel
  `lists`/`todos`
- `AGENTS.md` bagian "Aturan struktur" dan "Aturan kode"
- Task 02 (helper pagination di `platform/httpx` — dipakai bentuk filter di
  sini, bukan diciptakan ulang)

## File yang dibuat atau disentuh

| Path                                                                 | Isi                                                                     |
| -------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| `internal/modules/task/internal/domain/list.go`                      | entity `List`, method `Rename`                                          |
| `internal/modules/task/internal/domain/todo.go`                      | entity `Todo`, method `Complete`, `Reopen`, `MoveTo`                    |
| `internal/modules/task/internal/domain/status.go`                    | tipe `Status`, `Priority`                                               |
| `internal/modules/task/internal/domain/errors.go`                    | error sentinel                                                          |
| `internal/modules/task/internal/domain/repository.go`                | interface `ListRepository`, `TodoRepository`, struct filter             |
| `internal/modules/task/internal/domain/*_test.go`                    | unit test domain                                                        |
| `internal/modules/task/internal/application/create_list.go` dst      | satu file per usecase (atau dikelompokkan per agregat, lihat langkah 4) |
| `internal/modules/task/internal/application/dto.go`                  | input/output usecase                                                    |
| `internal/modules/task/internal/application/*_test.go`               | unit test usecase dengan fake repository                                |
| `internal/modules/task/internal/application/fake_repository_test.go` | fake `ListRepository`/`TodoRepository`                                  |

## Langkah

### 1. `domain/status.go` — `Status` dan `Priority` sebagai tipe

Jangan pakai `string` telanjang untuk status atau prioritas — kalau begitu,
`todo.Status = "selesai"` (typo) lolos compiler dan baru ketahuan di database
lewat CHECK constraint yang error-nya generik.

```go
package domain

type Status string

const (
	StatusTodo Status = "todo"
	StatusDone Status = "done"
)

func (s Status) Valid() bool {
	switch s {
	case StatusTodo, StatusDone:
		return true
	default:
		return false
	}
}

type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
)

func (p Priority) Valid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh:
		return true
	default:
		return false
	}
}
```

Nilai string harus sama persis dengan nilai CHECK constraint di migrasi task 01 — cek migrasinya sebelum menulis konstanta ini, jangan menebak.

### 2. `domain/errors.go`

```go
package domain

import "errors"

var (
	ErrListNotFound   = errors.New("list not found")
	ErrTodoNotFound   = errors.New("todo not found")
	ErrListNameTaken  = errors.New("list name already taken")
	ErrForbidden      = errors.New("forbidden")
	ErrInvalidStatus  = errors.New("invalid status")
	ErrInvalidPriority = errors.New("invalid priority")
	ErrEmptyTitle     = errors.New("title must not be empty")
	ErrTitleTooLong   = errors.New("title exceeds maximum length")
)
```

**`ErrForbidden` nyaris tidak pernah dipakai di modul ini — baca ini sampai
habis sebelum memakainya di mana pun.** Kalau user A meminta todo milik user
B, jawabannya **`ErrTodoNotFound`**, bukan `ErrForbidden`. Alasannya:
respons 403 memberi tahu penyerang "resource ini ADA, cuma kamu tidak boleh
lihat" — itu sudah kebocoran informasi (todo dengan ID itu ada di sistem).
Respons 404 tidak membocorkan apa pun: identik dengan "todo ini memang tidak
ada". Aturannya seragam di seluruh modul: **query selalu di-scope ke
`userID` sejak dari SQL** (lihat task 07), jadi dari sudut pandang
repository, todo milik orang lain memang tidak ada baris yang cocok — bukan
ada baris yang lalu ditolak. `ErrForbidden` disiapkan untuk kasus lain kalau
suatu saat muncul (misal validasi lintas-aggregate di usecase yang tidak bisa
diselesaikan di level query), tapi kalau sampai akhir task ini tidak pernah
dipakai, itu **benar**, bukan tanda kurang lengkap.

### 3. `domain/list.go` dan `domain/todo.go`

```go
package domain

import "time"

type List struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	Position  int32
	CreatedAt time.Time
	UpdatedAt time.Time
}

const maxListNameLen = 100

func (l *List) Rename(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrEmptyTitle
	}
	if len(name) > maxListNameLen {
		return ErrTitleTooLong
	}
	l.Name = name
	return nil
}
```

```go
type Todo struct {
	ID          uuid.UUID
	ListID      uuid.UUID
	UserID      uuid.UUID
	Title       string
	Notes       string
	Status      Status
	Priority    Priority
	DueAt       *time.Time
	CompletedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

const maxTitleLen = 200

func NewTodo(id, listID, userID uuid.UUID, title string, now time.Time) (*Todo, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, ErrEmptyTitle
	}
	if len(title) > maxTitleLen {
		return nil, ErrTitleTooLong
	}
	return &Todo{
		ID: id, ListID: listID, UserID: userID, Title: title,
		Status: StatusTodo, Priority: PriorityMedium,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

// Complete menandai todo selesai. Idempoten dengan sengaja: memanggil Complete
// pada todo yang sudah selesai tidak error dan tidak mengubah CompletedAt asli.
// Alasan: klien yang mengirim ulang request (retry jaringan, double-click)
// tidak boleh melihat error hanya karena aksinya sudah berhasil sebelumnya —
// "tandai selesai" adalah perintah yang menyatakan keadaan akhir yang
// diinginkan, bukan transisi satu-kali yang harus dijaga ketat seperti
// pembayaran. Kalau perilaku ini pernah dianggap salah, ganti jadi menolak
// dengan error baru (misal ErrAlreadyDone) dan perbarui test terkait.
func (t *Todo) Complete(now time.Time) {
	if t.Status == StatusDone {
		return
	}
	t.Status = StatusDone
	t.CompletedAt = &now
	t.UpdatedAt = now
}

func (t *Todo) Reopen(now time.Time) {
	if t.Status == StatusTodo {
		return
	}
	t.Status = StatusTodo
	t.CompletedAt = nil
	t.UpdatedAt = now
}

// MoveTo memindahkan todo ke list lain. WAJIB memperbarui Todo.UserID —
// bukan opsional, bukan optimasi. ADR-011 mendenormalisasi todos.user_id
// supaya query scoping tidak perlu join ke lists. Konsekuensinya: satu-
// satunya tempat user_id sebuah todo bisa berubah adalah di sini, dan kalau
// baris ini dilewatkan, todo yang baru dipindah ke list milik user lain
// (kasus mustahil karena langkah di bawah menolaknya duluan) atau — kasus
// yang benar-benar terjadi — todo tetap membawa UserID lama padahal listnya
// sudah pindah ke list lain milik user yang sama, membuat data todos.user_id
// dan todos.list_id→lists.user_id tidak sinkron. Ini titik paling mudah salah
// di seluruh modul task. Test wajib menegaskan baris ini (lihat langkah 5).
func (t *Todo) MoveTo(list *List, now time.Time) error {
	if list.UserID != t.UserID {
		return ErrForbidden
	}
	t.ListID = list.ID
	t.UserID = list.UserID
	t.UpdatedAt = now
	return nil
}
```

Catatan soal `MoveTo` dan `ErrForbidden`: penolakan di sini berbeda dari kasus
"todo milik user lain" pada butir 2. Di sini pemanggilnya (usecase
`MoveTodo`) sudah memuat todo dan list yang **sama-sama sudah lolos scoping
`userID` dari repository** — kalau list target ternyata `UserID`-nya beda,
itu berarti bug logika di usecase (usecase salah mengambil list), bukan upaya
mengakses resource orang lain. Karena itu domain menolak lewat panic-worthy
invariant, dan `ErrForbidden` di sini masuk akal dipakai — usecase tidak
pernah membiarkan userID list datang dari input yang tidak tervalidasi.
Usecase `MoveTodo` (langkah 4) tetap wajib memuat list lewat repository yang
di-scope ke `userID` pemanggil, sehingga kondisi ini pada praktiknya tidak
pernah tercapai lewat request user biasa — tapi domain tidak boleh
mengasumsikan itu, makanya dicek eksplisit.

### 4. `domain/repository.go` — port

```go
package domain

type ListFilter struct {
	UserID uuid.UUID
}

type TodoFilter struct {
	UserID     uuid.UUID
	ListID     *uuid.UUID
	Status     *Status
	DueBefore  *time.Time
	DueAfter   *time.Time
	Sort       TodoSort
	Cursor     *Cursor // lihat task 02: helper keyset pagination
	Limit      int32
}

type TodoSort string

const (
	SortDueAtAsc     TodoSort = "due_at_asc"
	SortPriorityDesc TodoSort = "priority_desc"
	SortCreatedDesc  TodoSort = "created_desc" // default
)

type ListRepository interface {
	Create(ctx context.Context, l *List) error
	GetByID(ctx context.Context, userID, id uuid.UUID) (*List, error)
	ListByUser(ctx context.Context, filter ListFilter) ([]*List, error)
	Update(ctx context.Context, l *List) error
	Delete(ctx context.Context, userID, id uuid.UUID) error
}

type TodoRepository interface {
	Create(ctx context.Context, t *Todo) error
	GetByID(ctx context.Context, userID, id uuid.UUID) (*Todo, error)
	Query(ctx context.Context, filter TodoFilter) ([]*Todo, error)
	Update(ctx context.Context, t *Todo) error
	Delete(ctx context.Context, userID, id uuid.UUID) error
}
```

Setiap method menerima `userID` eksplisit, bahkan yang menerima `id` — ini
menegakkan di level signature bahwa tidak ada cara memanggil repository tanpa
scoping. Implementasi Postgres (task 07) mewajibkan `WHERE user_id = $1` di
setiap query yang mengimplementasikan interface ini.

Bentuk `Cursor` dan konvensi keyset ikuti helper pagination task 02 — jangan
membuat tipe pagination baru di modul ini.

### 5. `application/` — usecase

Satu struct usecase per aksi, konstruktor `New<Nama>(repo..., clock clock.Clock, idgen id.Generator)`. Semua usecase menerima `userID uuid.UUID` sebagai
parameter method eksplisit (bukan lewat context, bukan lewat field struct
yang di-set sekali):

```go
type CreateTodo struct {
	todos TodoRepository
	lists ListRepository
	clock clock.Clock
	newID func() (uuid.UUID, error)
}

func (uc *CreateTodo) Execute(ctx context.Context, userID uuid.UUID, in CreateTodoInput) (*domain.Todo, error) {
	list, err := uc.lists.GetByID(ctx, userID, in.ListID)
	if err != nil {
		return nil, fmt.Errorf("get list: %w", err)
	}
	id, err := uc.newID()
	if err != nil {
		return nil, fmt.Errorf("generate id: %w", err)
	}
	todo, err := domain.NewTodo(id, list.ID, userID, in.Title, uc.clock.Now())
	if err != nil {
		return nil, err
	}
	if err := uc.todos.Create(ctx, todo); err != nil {
		return nil, fmt.Errorf("create todo: %w", err)
	}
	return todo, nil
}
```

**Kenapa `userID` parameter eksplisit, bukan diambil usecase sendiri dari
context:** usecase adalah lapisan `application`, dan `AGENTS.md` melarang
`domain` tahu apa pun soal dunia luar — batas yang sama berlaku secara
praktik untuk `application` soal HTTP. Kalau usecase membaca
`ctx.Value("userID")` sendiri, dia diam-diam mengasumsikan pemanggilnya
adalah HTTP middleware tertentu, dan unit test usecase (yang tidak
melewati HTTP sama sekali) harus memalsukan context itu hanya untuk
memuaskan asumsi yang tidak relevan dengan aturan bisnis yang sedang diuji.
Dengan `userID` sebagai parameter biasa, test cukup memanggil
`uc.Execute(ctx, someUUID, input)` — jelas, eksplisit, tidak ada context
magic. Handler HTTP (task 07) yang bertanggung jawab menarik `userID` dari
context dan meneruskannya sebagai argumen.

Usecase wajib ada (method `Execute`, nama field boleh menyesuaikan):

- `CreateList`, `ListLists`, `RenameList`, `DeleteList`
- `CreateTodo`, `ListTodos`, `GetTodo`, `UpdateTodo`, `DeleteTodo`
- `CompleteTodo`, `ReopenTodo`, `MoveTodo`

`UpdateTodo` menerima input dengan field pointer/opsional (`*string` untuk
`Notes`, `*Priority` untuk `Priority`, dst) supaya "field tidak dikirim"
bisa dibedakan dari "field dikirim kosong" — jangan menyelesaikan masalah ini
di usecase kalau HTTP-nya belum ada; cukup terima input yang sudah berbentuk
pointer dan terapkan field mana pun yang tidak nil. Pembahasan lengkap
soal PATCH parsial ada di task 07 — di sini cukup pastikan usecase sudah bisa
menerima bentuk input itu.

`MoveTodo.Execute` memuat todo lewat `todos.GetByID(ctx, userID, todoID)` dan
list tujuan lewat `lists.GetByID(ctx, userID, listID)` — **keduanya di-scope
`userID` pemanggil**, baru memanggil `todo.MoveTo(list, now)`. Kalau list
tujuan tidak ditemukan lewat scoping ini, error yang keluar adalah
`ErrListNotFound`, bukan `ErrForbidden` — user tidak pernah tahu apakah list
ID yang dia coba pakai benar-benar ada milik orang lain.

Boleh mengelompokkan usecase per file (`list_usecases.go`,
`todo_usecases.go`) atau satu file satu usecase — pilih salah satu dan
konsisten, jangan campur.

### 6. Fake repository dan test

Fake repository: struct dengan `map[uuid.UUID]*domain.List` /
`map[uuid.UUID]*domain.Todo` sebagai penyimpanan, method-nya meniru perilaku
scoping Postgres asli (termasuk mengembalikan not-found kalau `userID` tidak
cocok — ini bagian yang paling penting ditiru dengan benar, karena kalau fake
tidak meniru scoping, test "user lain dapat not-found" jadi palsu-hijau).

```go
type fakeTodoRepo struct {
	mu    sync.Mutex
	items map[uuid.UUID]*domain.Todo
}

func (r *fakeTodoRepo) GetByID(ctx context.Context, userID, id uuid.UUID) (*domain.Todo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.items[id]
	if !ok || t.UserID != userID {
		return nil, domain.ErrTodoNotFound
	}
	cp := *t
	return &cp, nil
}
```

Kasus test wajib (di `application`, memakai fake repo + `clock.Fixed`):

1. **Buat todo di list orang lain → not found.** `CreateTodo.Execute` dengan
   `userID` A tapi `ListID` milik user B mengembalikan `ErrListNotFound`
   (bukan `ErrForbidden`).
2. **Pindah todo antar list ikut memperbarui `UserID`.** Setelah
   `MoveTodo.Execute` sukses, ambil todo dari fake repo dan assert
   `todo.UserID == listTujuan.UserID`, bukan cuma `todo.ListID` yang berubah.
   Ini test yang paling wajib ada di seluruh task — lihat catatan di langkah 3.
3. **Complete lalu Reopen.** `CompleteTodo` mengisi `CompletedAt`,
   `ReopenTodo` mengosongkannya lagi; panggil `Complete` dua kali berturut
   dan pastikan `CompletedAt` dari panggilan pertama tidak berubah (bukti
   idempoten, bukan cuma "tidak error").
4. **Nama list duplikat.** `CreateList` atau `RenameList` dengan nama yang
   sudah dipakai user yang sama mengembalikan `ErrListNameTaken` — di layer
   usecase ini disimulasikan lewat fake repo yang mengecek keunikan sendiri
   dan mengembalikan error itu (di Postgres asli ini datang dari constraint
   unique, dipetakan di task 07 — fake repo di sini meniru kontraknya, bukan
   mekanismenya).

Test domain murni (tanpa fake repo, langsung memanggil method `Todo`/`List`):
`NewTodo` menolak judul kosong/whitespace-only, `NewTodo` menolak judul
melebihi `maxTitleLen`, `MoveTo` menolak kalau `list.UserID != todo.UserID`.

## Kriteria selesai

```bash
cd apps/go-chi-api
go build ./internal/modules/task/...
go vet ./internal/modules/task/...
go test ./internal/modules/task/... -short -v
```

- Semua kriteria di atas hijau.
- `go list -deps ./internal/modules/task/... | grep -E 'net/http|chi|pgx|sqlc'`
  tidak menghasilkan apa pun — modul `task` belum menyentuh HTTP atau
  database sama sekali di task ini.
- `go list -deps ./internal/modules/task/... | grep 'modules/identity'` tidak
  menghasilkan apa pun — pembuktian eksplisit ADR-003.
- Test untuk "pindah list memperbarui UserID" ada dan lulus (grep nama
  testnya di file test, pastikan bukan sekadar dibuat lalu tidak dipanggil).
- Test untuk "buat todo di list orang lain → not found" ada dan
  mengembalikan `domain.ErrListNotFound`, bukan `domain.ErrForbidden`.
- `gofmt -l internal/modules/task` tidak menghasilkan output.

## Jebakan

- **`MoveTo` yang lupa mengubah `UserID`** adalah bug yang lolos semua test
  kalau test hanya mengecek `ListID`. Test wajib membaca `UserID` setelah
  pindah, bukan cuma `ListID`.
- Jangan taruh `ErrForbidden` sebagai hasil query "resource milik user lain"
  di mana pun — itu selalu `ErrListNotFound`/`ErrTodoNotFound`. Baca ulang
  langkah 2 kalau ragu.
- Fake repository yang tidak meniru scoping `userID` (misal `GetByID` yang
  mengabaikan parameter `userID`-nya) membuat seluruh test isolasi jadi
  palsu-hijau — repository asli di task 07 akan tetap benar tapi test di
  task ini tidak membuktikan apa-apa. Selalu assert lewat perilaku, bukan
  lewat asumsi bahwa fake pasti benar.
- `Complete`/`Reopen` menerima `now time.Time` dari pemanggil (usecase, yang
  dapat dari `clock.Clock`), bukan memanggil `time.Now()` sendiri — domain
  tidak boleh tahu soal waktu nyata, lihat aturan di `AGENTS.md`.
- Jangan membuat validasi "nama list unik" jadi query langsung di usecase
  (`SELECT ... WHERE name = ...` manual) — itu tugas constraint database plus
  pemetaan error di task 07. Usecase di task ini cukup meneruskan error yang
  dikembalikan repository.
- `UpdateTodo` dengan field pointer nil berarti "tidak diubah" — kalau usecase
  keliru menganggap pointer nil sebagai "set ke zero value", data existing
  akan tertimpa kosong. Test path ini juga kalau sempat, meski test wajibnya
  ada di task 07 (level HTTP).
