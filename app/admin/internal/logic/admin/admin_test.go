package admin

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/os/gtime"

	"github.com/hughtaiyaki/goframeshop/app/admin/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/admin/internal/service"
)

type fakeAdminRepo struct {
	rows   map[uint64]*entity.AdminInfo
	nextID uint64
}

func newFakeAdminRepo() *fakeAdminRepo {
	return &fakeAdminRepo{rows: make(map[uint64]*entity.AdminInfo)}
}

func (f *fakeAdminRepo) Create(_ context.Context, a *entity.AdminInfo) (uint64, error) {
	for _, row := range f.rows {
		if row.Name == a.Name {
			return 0, service.ErrAdminExists
		}
	}
	f.nextID++
	stored := *a
	stored.Id = f.nextID
	f.rows[f.nextID] = &stored
	return f.nextID, nil
}

// The fakes mirror GoFrame's automatic soft-delete filter.
func (f *fakeAdminRepo) FindByName(_ context.Context, name string) (*entity.AdminInfo, error) {
	for _, row := range f.rows {
		if row.Name == name && row.DeletedAt == nil {
			copied := *row
			return &copied, nil
		}
	}
	return nil, nil
}

func (f *fakeAdminRepo) FindByID(_ context.Context, id uint64) (*entity.AdminInfo, error) {
	if row, ok := f.rows[id]; ok && row.DeletedAt == nil {
		copied := *row
		return &copied, nil
	}
	return nil, nil
}

const strongPassword = "correct-horse-battery"

func TestBootstrapThenLogin(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAdminRepo()
	svc := New(repo)

	id, err := svc.Bootstrap(ctx, " root ", strongPassword)
	if err != nil {
		t.Fatal(err)
	}
	stored := repo.rows[id]
	if stored.Name != "root" || stored.IsSuper != 1 || strings.Contains(stored.PasswordHash, strongPassword) {
		t.Fatalf("stored %+v", stored)
	}

	admin, err := svc.Login(ctx, "root", strongPassword)
	if err != nil || admin.ID != id || !admin.IsSuper {
		t.Fatalf("Login = %+v, %v", admin, err)
	}
	// Unknown name and wrong password are indistinguishable.
	for _, attempt := range [][2]string{{"root", "wrong-password-123"}, {"nobody", strongPassword}} {
		if _, err := svc.Login(ctx, attempt[0], attempt[1]); !errors.Is(err, service.ErrInvalidCredential) {
			t.Errorf("Login(%q): err = %v, want ErrInvalidCredential", attempt[0], err)
		}
	}
}

func TestOnlyALiveSuperAdminCanCreate(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAdminRepo()
	svc := New(repo)
	root, _ := svc.Bootstrap(ctx, "root", strongPassword)

	staff, err := svc.Create(ctx, root, &service.CreateInput{Name: "staff", Password: strongPassword})
	if err != nil {
		t.Fatal(err)
	}
	if repo.rows[staff].IsSuper != 0 {
		t.Fatal("isSuper defaulted to true")
	}
	if _, err := svc.Create(ctx, staff, &service.CreateInput{Name: "x", Password: strongPassword}); !errors.Is(err, service.ErrPermissionDenied) {
		t.Fatalf("ordinary admin: err = %v, want ErrPermissionDenied", err)
	}
	if _, err := svc.Create(ctx, 999, &service.CreateInput{Name: "x", Password: strongPassword}); !errors.Is(err, service.ErrPermissionDenied) {
		t.Fatalf("unknown caller: err = %v, want ErrPermissionDenied", err)
	}
	// A super admin deleted after logging in loses the right immediately,
	// even though their token would still verify.
	repo.rows[root].DeletedAt = gtime.Now()
	if _, err := svc.Create(ctx, root, &service.CreateInput{Name: "x", Password: strongPassword}); !errors.Is(err, service.ErrPermissionDenied) {
		t.Fatalf("deleted super admin: err = %v, want ErrPermissionDenied", err)
	}
	if _, err := svc.Login(ctx, "root", strongPassword); !errors.Is(err, service.ErrInvalidCredential) {
		t.Fatalf("deleted admin logged in: %v", err)
	}
}

func TestCreateValidatesInput(t *testing.T) {
	ctx := context.Background()
	svc := New(newFakeAdminRepo())
	root, _ := svc.Bootstrap(ctx, "root", strongPassword)

	bad := map[string]*service.CreateInput{
		"short password": {Name: "a", Password: "elevenchars"},
		"long password":  {Name: "a", Password: strings.Repeat("x", 129)},
		"blank name":     {Name: "   ", Password: strongPassword},
		"long name":      {Name: strings.Repeat("名", 65), Password: strongPassword},
	}
	for name, in := range bad {
		if _, err := svc.Create(ctx, root, in); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("%s: err = %v, want ErrInvalidArgument", name, err)
		}
	}
	if _, err := svc.Create(ctx, root, &service.CreateInput{Name: "root", Password: strongPassword}); !errors.Is(err, service.ErrAdminExists) {
		t.Fatalf("duplicate: err = %v, want ErrAdminExists", err)
	}
	if _, err := svc.Bootstrap(ctx, "weak", "short"); !errors.Is(err, service.ErrInvalidArgument) {
		t.Fatalf("bootstrap with weak password: err = %v", err)
	}
}
