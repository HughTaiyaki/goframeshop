package user

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/os/gtime"

	"github.com/hughtaiyaki/goframeshop/app/user/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/service"
)

// fakeUserRepo is an in-memory stand-in for the user table.
type fakeUserRepo struct {
	byID   map[uint64]*entity.UserInfo
	nextID uint64
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{byID: make(map[uint64]*entity.UserInfo)}
}

func (f *fakeUserRepo) Create(_ context.Context, u *entity.UserInfo) (uint64, error) {
	f.nextID++
	stored := *u
	stored.Id = f.nextID
	stored.CreatedAt = gtime.Now()
	f.byID[f.nextID] = &stored
	return f.nextID, nil
}

func (f *fakeUserRepo) FindByName(_ context.Context, name string) (*entity.UserInfo, error) {
	for _, record := range f.byID {
		if record.Name == name && record.DeletedAt == nil {
			copied := *record
			return &copied, nil
		}
	}
	return nil, nil
}

func (f *fakeUserRepo) FindByID(_ context.Context, id uint64) (*entity.UserInfo, error) {
	record, ok := f.byID[id]
	if !ok || record.DeletedAt != nil {
		return nil, nil
	}
	copied := *record
	return &copied, nil
}

func (f *fakeUserRepo) UpdateCredential(_ context.Context, id uint64, hash string, salt []byte) error {
	record, ok := f.byID[id]
	if !ok {
		return nil
	}
	record.PasswordHash = hash
	record.PasswordSalt = salt
	return nil
}

func (f *fakeUserRepo) UpdateProfile(_ context.Context, id uint64, avatar string, sex uint, sign string) error {
	record, ok := f.byID[id]
	if !ok {
		return nil
	}
	record.Avatar, record.Sex, record.Sign = avatar, sex, sign
	return nil
}

const goodPassword = "correct-horse"

func TestRegisterThenLogin(t *testing.T) {
	ctx := context.Background()
	svc := New(newFakeUserRepo())

	created, err := svc.Register(ctx, &service.RegisterInput{Username: "alice", Password: goodPassword})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 {
		t.Fatal("expected a non-zero user id")
	}
	// The domain type must not carry any credential material.
	if created.Name != "alice" {
		t.Fatalf("name = %q, want alice", created.Name)
	}

	account, err := svc.Login(ctx, "alice", goodPassword)
	if err != nil {
		t.Fatal(err)
	}
	if account.ID != created.ID {
		t.Fatalf("login returned id %d, want %d", account.ID, created.ID)
	}
}

func TestRegisterRejectsDuplicateAndWeakPassword(t *testing.T) {
	ctx := context.Background()
	svc := New(newFakeUserRepo())
	if _, err := svc.Register(ctx, &service.RegisterInput{Username: "alice", Password: goodPassword}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register(ctx, &service.RegisterInput{Username: "alice", Password: goodPassword}); !errors.Is(err, service.ErrUserExists) {
		t.Fatalf("duplicate register: err = %v, want ErrUserExists", err)
	}
	if _, err := svc.Register(ctx, &service.RegisterInput{Username: "bob", Password: "short"}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Fatalf("weak password: err = %v, want ErrInvalidArgument", err)
	}
}

func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	ctx := context.Background()
	svc := New(newFakeUserRepo())
	if _, err := svc.Register(ctx, &service.RegisterInput{Username: "alice", Password: goodPassword}); err != nil {
		t.Fatal(err)
	}

	// A wrong password and an unknown username must produce the same error,
	// otherwise the endpoint becomes a username enumeration oracle.
	if _, err := svc.Login(ctx, "alice", "wrong-password"); !errors.Is(err, service.ErrInvalidCredential) {
		t.Fatalf("wrong password: err = %v, want ErrInvalidCredential", err)
	}
	if _, err := svc.Login(ctx, "nobody", goodPassword); !errors.Is(err, service.ErrInvalidCredential) {
		t.Fatalf("unknown user: err = %v, want ErrInvalidCredential", err)
	}
}

func TestGetProfile(t *testing.T) {
	ctx := context.Background()
	svc := New(newFakeUserRepo())
	created, err := svc.Register(ctx, &service.RegisterInput{Username: "alice", Password: goodPassword})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetProfile(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetProfile(ctx, created.ID+1000); !errors.Is(err, service.ErrUserNotFound) {
		t.Fatalf("unknown id: err = %v, want ErrUserNotFound", err)
	}
}

func TestChangePasswordRequiresCurrentPassword(t *testing.T) {
	ctx := context.Background()
	svc := New(newFakeUserRepo())
	created, err := svc.Register(ctx, &service.RegisterInput{Username: "alice", Password: goodPassword})
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.ChangePassword(ctx, created.ID, "not-my-password", "another-good-one"); !errors.Is(err, service.ErrInvalidCredential) {
		t.Fatalf("wrong current password: err = %v, want ErrInvalidCredential", err)
	}

	const newPassword = "another-good-one"
	if err := svc.ChangePassword(ctx, created.ID, goodPassword, newPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login(ctx, "alice", goodPassword); !errors.Is(err, service.ErrInvalidCredential) {
		t.Fatal("old password still works after change")
	}
	if _, err := svc.Login(ctx, "alice", newPassword); err != nil {
		t.Fatalf("new password rejected: %v", err)
	}
}

func TestStoredPasswordIsNotReversible(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	svc := New(repo)
	if _, err := svc.Register(ctx, &service.RegisterInput{Username: "alice", Password: goodPassword}); err != nil {
		t.Fatal(err)
	}
	stored := repo.byID[1]
	if stored.PasswordHash == goodPassword {
		t.Fatal("password stored in plain text")
	}
	if len(stored.PasswordSalt) == 0 {
		t.Fatal("no salt was generated")
	}
}

func TestUpdateProfileReplacesEditableFields(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	svc := New(repo)
	created, err := svc.Register(ctx, &service.RegisterInput{
		Username: "alice", Password: goodPassword, Avatar: "https://cdn.example.com/a.png", Sex: 1, Sign: "hi",
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := svc.UpdateProfile(ctx, created.ID, &service.ProfileInput{
		Avatar: "  https://cdn.example.com/b.png ", Sex: 2, Sign: " new sign ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Avatar != "https://cdn.example.com/b.png" || updated.Sex != 2 || updated.Sign != "new sign" {
		t.Fatalf("returned profile = %+v", updated)
	}
	if updated.Name != "alice" || updated.Status != statusActive {
		t.Fatalf("username or status changed: %+v", updated)
	}
	stored := repo.byID[created.ID]
	if stored.Avatar != updated.Avatar || stored.Sex != 2 || stored.Sign != "new sign" {
		t.Fatalf("stored row = %+v", stored)
	}

	// A full replacement: empty fields clear the old values.
	if _, err := svc.UpdateProfile(ctx, created.ID, &service.ProfileInput{}); err != nil {
		t.Fatal(err)
	}
	if stored := repo.byID[created.ID]; stored.Avatar != "" || stored.Sex != 0 || stored.Sign != "" {
		t.Fatalf("fields not cleared: %+v", stored)
	}

	// The password is untouched.
	if _, err := svc.Login(ctx, "alice", goodPassword); err != nil {
		t.Fatalf("login after profile update: %v", err)
	}
}

func TestProfileRejectsBadFields(t *testing.T) {
	ctx := context.Background()
	svc := New(newFakeUserRepo())
	created, err := svc.Register(ctx, &service.RegisterInput{Username: "alice", Password: goodPassword})
	if err != nil {
		t.Fatal(err)
	}
	bad := map[string]*service.ProfileInput{
		"javascript avatar": {Avatar: "javascript:alert(1)"},
		"data avatar":       {Avatar: "data:image/png;base64,AAAA"},
		"relative avatar":   {Avatar: "/a.png"},
		"avatar with user":  {Avatar: "https://u:p@example.com/a.png"},
		"avatar control":    {Avatar: "https://example.com/a\n.png"},
		"avatar too long":   {Avatar: "https://example.com/" + strings.Repeat("a", service.MaxAvatarBytes)},
		"sex out of range":  {Sex: 3},
		"sign too long":     {Sign: strings.Repeat("签", service.MaxSignRunes+1)},
	}
	for name, in := range bad {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.UpdateProfile(ctx, created.ID, in); !errors.Is(err, service.ErrInvalidArgument) {
				t.Fatalf("update: err = %v, want ErrInvalidArgument", err)
			}
			_, err := svc.Register(ctx, &service.RegisterInput{
				Username: "bob", Password: goodPassword, Avatar: in.Avatar, Sex: in.Sex, Sign: in.Sign,
			})
			if !errors.Is(err, service.ErrInvalidArgument) {
				t.Fatalf("register: err = %v, want ErrInvalidArgument", err)
			}
		})
	}
	// The longest allowed sign is accepted.
	if _, err := svc.UpdateProfile(ctx, created.ID, &service.ProfileInput{Sign: strings.Repeat("签", service.MaxSignRunes)}); err != nil {
		t.Fatalf("max sign: %v", err)
	}
}

func TestUpdateProfileOfUnknownUser(t *testing.T) {
	svc := New(newFakeUserRepo())
	if _, err := svc.UpdateProfile(context.Background(), 404, &service.ProfileInput{}); !errors.Is(err, service.ErrUserNotFound) {
		t.Fatalf("err = %v, want ErrUserNotFound", err)
	}
}
