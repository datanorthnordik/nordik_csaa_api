package auth

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"nordikcsaaapi/internal/util"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestAuthServiceReturnsStoreUnavailableWithoutDB(t *testing.T) {
	service := &AuthService{}

	if _, err := service.CreateUser(Auth{}); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected CreateUser to return ErrStoreUnavailable, got %v", err)
	}

	if _, err := service.GetUser("ada@example.com"); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected GetUser to return ErrStoreUnavailable, got %v", err)
	}

	if _, err := service.GetUserByID(42); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected GetUserByID to return ErrStoreUnavailable, got %v", err)
	}
	if _, err := service.CreatePasswordResetOTP("ada@example.com"); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected CreatePasswordResetOTP to return ErrStoreUnavailable, got %v", err)
	}
	if _, err := service.VerifyPasswordResetOTP("ada@example.com", "123456", "newSecret123"); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected VerifyPasswordResetOTP to return ErrStoreUnavailable, got %v", err)
	}
	if _, err := service.GetUnusedOTPByEmail("ada@example.com"); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("expected GetUnusedOTPByEmail to return ErrStoreUnavailable, got %v", err)
	}
}

func TestCreatePasswordResetOTPSuccessAndUserNotFound(t *testing.T) {
	db, mock, cleanup := setupAuthMockDB(t)
	defer cleanup()

	service := &AuthService{DB: db}
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users" WHERE email = $1 ORDER BY "users"."id" LIMIT $2`)).
		WithArgs("ada@example.com", 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "firstname", "lastname", "email", "password", "role", "created_at", "updated_at",
		}).AddRow(
			7, "Ada", "Lovelace", "ada@example.com", "hashed-password", "User", now, now,
		))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "password_reset_otps"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(13))
	mock.ExpectCommit()

	reset, err := service.CreatePasswordResetOTP("ada@example.com")
	if err != nil {
		t.Fatalf("CreatePasswordResetOTP returned error: %v", err)
	}
	if reset.ID != 13 || reset.UserID != 7 || reset.Email != "ada@example.com" || reset.IsUsed {
		t.Fatalf("unexpected password reset OTP: %#v", reset)
	}
	if !regexp.MustCompile(`^\d{6}$`).MatchString(reset.OTP) {
		t.Fatalf("expected generated OTP to be 6 digits, got %q", reset.OTP)
	}
	if reset.ExpiresAt.Before(now.Add(9*time.Minute)) || reset.ExpiresAt.After(now.Add(11*time.Minute)) {
		t.Fatalf("expected OTP expiry to be about 10 minutes in the future, got %v", reset.ExpiresAt)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users" WHERE email = $1 ORDER BY "users"."id" LIMIT $2`)).
		WithArgs("missing@example.com", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	if _, err := service.CreatePasswordResetOTP("missing@example.com"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestVerifyPasswordResetOTPSuccessUpdatesPasswordAndMarksOTPUsed(t *testing.T) {
	db, mock, cleanup := setupAuthMockDB(t)
	defer cleanup()

	service := &AuthService{DB: db}
	now := time.Now()
	oldPassword, err := util.HashPassword("oldSecret123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	mock.ExpectQuery(`SELECT \* FROM "password_reset_otps" WHERE email = \$1 AND otp = \$2 ORDER BY created_at DESC.*LIMIT \$3`).
		WithArgs("ada@example.com", "123456", 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "email", "otp", "expires_at", "is_used", "created_at", "updated_at",
		}).AddRow(
			13, 7, "ada@example.com", "123456", now.Add(10*time.Minute), false, now.Add(-time.Minute), now.Add(-time.Minute),
		))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users" WHERE email = $1 ORDER BY "users"."id" LIMIT $2`)).
		WithArgs("ada@example.com", 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "firstname", "lastname", "email", "password", "role", "created_at", "updated_at",
		}).AddRow(
			7, "Ada", "Lovelace", "ada@example.com", oldPassword, "User", now.Add(-24*time.Hour), now.Add(-time.Hour),
		))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "users" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "password_reset_otps" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "password_reset_otps" WHERE user_id = $1 AND expires_at < $2`)).
		WithArgs(7, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	user, err := service.VerifyPasswordResetOTP("ada@example.com", "123456", "newSecret123")
	if err != nil {
		t.Fatalf("VerifyPasswordResetOTP returned error: %v", err)
	}
	if user.ID != 7 || user.Email != "ada@example.com" {
		t.Fatalf("unexpected user returned from password reset: %#v", user)
	}
	if user.Password == oldPassword {
		t.Fatal("expected returned user password hash to be updated")
	}
	if err := util.VerifyPassword("newSecret123", user.Password); err != nil {
		t.Fatalf("expected updated password hash to verify, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestVerifyPasswordResetOTPRejectsInvalidUsedAndExpiredOTPs(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(sqlmock.Sqlmock)
		wantErr   error
	}{
		{
			name: "invalid otp",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT \* FROM "password_reset_otps" WHERE email = \$1 AND otp = \$2 ORDER BY created_at DESC.*LIMIT \$3`).
					WithArgs("ada@example.com", "123456", 1).
					WillReturnError(gorm.ErrRecordNotFound)
			},
			wantErr: ErrInvalidOTP,
		},
		{
			name: "used otp",
			setupMock: func(mock sqlmock.Sqlmock) {
				now := time.Now()
				mock.ExpectQuery(`SELECT \* FROM "password_reset_otps" WHERE email = \$1 AND otp = \$2 ORDER BY created_at DESC.*LIMIT \$3`).
					WithArgs("ada@example.com", "123456", 1).
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "user_id", "email", "otp", "expires_at", "is_used", "created_at", "updated_at",
					}).AddRow(
						13, 7, "ada@example.com", "123456", now.Add(10*time.Minute), true, now.Add(-time.Minute), now.Add(-time.Minute),
					))
			},
			wantErr: ErrOTPAlreadyUsed,
		},
		{
			name: "expired otp",
			setupMock: func(mock sqlmock.Sqlmock) {
				now := time.Now()
				mock.ExpectQuery(`SELECT \* FROM "password_reset_otps" WHERE email = \$1 AND otp = \$2 ORDER BY created_at DESC.*LIMIT \$3`).
					WithArgs("ada@example.com", "123456", 1).
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "user_id", "email", "otp", "expires_at", "is_used", "created_at", "updated_at",
					}).AddRow(
						13, 7, "ada@example.com", "123456", now.Add(-time.Minute), false, now.Add(-2*time.Minute), now.Add(-2*time.Minute),
					))
			},
			wantErr: ErrOTPExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, cleanup := setupAuthMockDB(t)
			defer cleanup()

			tt.setupMock(mock)
			service := &AuthService{DB: db}

			if _, err := service.VerifyPasswordResetOTP("ada@example.com", "123456", "newSecret123"); !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet sql expectations: %v", err)
			}
		})
	}
}

func TestGetUnusedOTPByEmailSuccessAndInvalidOTP(t *testing.T) {
	db, mock, cleanup := setupAuthMockDB(t)
	defer cleanup()

	service := &AuthService{DB: db}
	now := time.Now()

	mock.ExpectQuery(`SELECT \* FROM "password_reset_otps" WHERE email = \$1 AND is_used = \$2 AND expires_at > \$3 ORDER BY created_at DESC.*LIMIT \$4`).
		WithArgs("ada@example.com", false, sqlmock.AnyArg(), 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "email", "otp", "expires_at", "is_used", "created_at", "updated_at",
		}).AddRow(
			13, 7, "ada@example.com", "123456", now.Add(10*time.Minute), false, now.Add(-time.Minute), now.Add(-time.Minute),
		))

	reset, err := service.GetUnusedOTPByEmail("ada@example.com")
	if err != nil {
		t.Fatalf("GetUnusedOTPByEmail returned error: %v", err)
	}
	if reset.Email != "ada@example.com" || reset.OTP != "123456" || reset.IsUsed {
		t.Fatalf("unexpected unused OTP result: %#v", reset)
	}

	mock.ExpectQuery(`SELECT \* FROM "password_reset_otps" WHERE email = \$1 AND is_used = \$2 AND expires_at > \$3 ORDER BY created_at DESC.*LIMIT \$4`).
		WithArgs("missing@example.com", false, sqlmock.AnyArg(), 1).
		WillReturnError(gorm.ErrRecordNotFound)

	if _, err := service.GetUnusedOTPByEmail("missing@example.com"); !errors.Is(err, ErrInvalidOTP) {
		t.Fatalf("expected ErrInvalidOTP, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func setupAuthMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 sqlDB,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	return db, mock, func() {
		_ = sqlDB.Close()
	}
}
