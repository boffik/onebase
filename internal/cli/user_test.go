package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ivantit66/onebase/internal/auth"
	"github.com/ivantit66/onebase/internal/storage"
	"github.com/spf13/cobra"
)

// userTestCmd собирает команду со всеми флагами, которые читают RunE-функции
// user-подкоманд (базовые + пароль/имя/админ), с выбранными project и sqlite.
func userTestCmd(t *testing.T, projectDir, dbPath string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	fs := cmd.Flags()
	fs.String("id", "", "")
	fs.String("project", ".", "")
	fs.String("sqlite", "", "")
	fs.String("db", "", "")
	fs.String("name", "", "")
	fs.Bool("admin", false, "")
	fs.Bool("generate", false, "")
	fs.Bool("password-stdin", false, "")
	mustSet(t, fs, "project", projectDir)
	mustSet(t, fs, "sqlite", dbPath)
	return cmd
}

func mustSet(t *testing.T, fs interface{ Set(string, string) error }, name, val string) {
	t.Helper()
	if err := fs.Set(name, val); err != nil {
		t.Fatalf("set %s=%s: %v", name, val, err)
	}
}

func TestUserCLI_AddListRoleInvariants(t *testing.T) {
	dir := t.TempDir()
	rolesDir := filepath.Join(dir, "roles")
	if err := os.MkdirAll(rolesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	role := "name: Кладовщик\npermissions:\n  documents:\n    Реализация: [read, post]\n"
	if err := os.WriteFile(filepath.Join(rolesDir, "warehouse.yaml"), []byte(role), 0o644); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(t.TempDir(), "users.db")

	// Первый пользователь обязан быть администратором.
	cmd := userTestCmd(t, dir, dbPath)
	mustSet(t, cmd.Flags(), "generate", "true")
	if err := runUserAdd(cmd, []string{"klad"}); err == nil {
		t.Fatal("первый пользователь без --admin должен быть отклонён (ErrFirstUserMustBeAdmin)")
	}

	// Админ создаётся.
	cmd = userTestCmd(t, dir, dbPath)
	mustSet(t, cmd.Flags(), "admin", "true")
	mustSet(t, cmd.Flags(), "generate", "true")
	if err := runUserAdd(cmd, []string{"admin"}); err != nil {
		t.Fatalf("создание админа: %v", err)
	}

	// Теперь можно завести обычного пользователя.
	cmd = userTestCmd(t, dir, dbPath)
	mustSet(t, cmd.Flags(), "generate", "true")
	if err := runUserAdd(cmd, []string{"klad"}); err != nil {
		t.Fatalf("создание пользователя: %v", err)
	}

	// Дубликат логина отклоняется (UNIQUE login).
	cmd = userTestCmd(t, dir, dbPath)
	mustSet(t, cmd.Flags(), "generate", "true")
	if err := runUserAdd(cmd, []string{"klad"}); err == nil {
		t.Fatal("повторный логин должен быть отклонён")
	}

	// Назначение роли из roles/*.yaml.
	cmd = userTestCmd(t, dir, dbPath)
	if err := runUserRoleAssign(cmd, []string{"klad", "Кладовщик"}); err != nil {
		t.Fatalf("назначение роли: %v", err)
	}

	// Проверяем состояние через репозиторий.
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := auth.NewRepo(db)

	users, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Fatalf("ожидалось 2 пользователя, получено %d", len(users))
	}
	u, err := findUserByLogin(ctx, repo, "klad")
	if err != nil {
		t.Fatal(err)
	}
	roles, err := repo.GetRolesForUser(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 || roles[0].Name != "Кладовщик" {
		t.Fatalf("ожидалась роль Кладовщик, получено %+v", roles)
	}

	// Снятие роли.
	cmd = userTestCmd(t, dir, dbPath)
	if err := runUserRoleRevoke(cmd, []string{"klad", "Кладовщик"}); err != nil {
		t.Fatalf("снятие роли: %v", err)
	}
	roles, _ = repo.GetRolesForUser(ctx, u.ID)
	if len(roles) != 0 {
		t.Fatalf("роль должна быть снята, осталось %+v", roles)
	}

	// Нельзя удалить последнего администратора.
	cmd = userTestCmd(t, dir, dbPath)
	if err := runUserRm(cmd, []string{"admin"}); err == nil {
		t.Fatal("удаление последнего админа должно быть отклонено (ErrLastAdmin)")
	}
}

func TestGeneratePassword(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		pw, err := generatePassword(16)
		if err != nil {
			t.Fatal(err)
		}
		if len(pw) != 16 {
			t.Fatalf("длина пароля %d, ожидалось 16", len(pw))
		}
		if seen[pw] {
			t.Fatalf("сгенерирован повторяющийся пароль %q", pw)
		}
		seen[pw] = true
		for _, c := range pw {
			if !containsRune(passwordAlphabet, c) {
				t.Fatalf("символ %q вне алфавита", c)
			}
		}
	}
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}
