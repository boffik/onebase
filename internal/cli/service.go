package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/ivantit66/onebase/internal/launcher"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage onebase as a system service",
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install onebase as a system service (systemd on Linux, sc.exe on Windows)",
	Example: `  onebase service install --id <base-id>
  onebase service install --db "postgres://..." --port 8080 --name myapp`,
	RunE: runServiceInstall,
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the onebase system service",
	Example: `  onebase service uninstall --name onebase-myapp`,
	RunE: runServiceUninstall,
}

func init() {
	serviceInstallCmd.Flags().String("id", "", "base ID from ibases registry")
	serviceInstallCmd.Flags().String("name", "", "service name (default: onebase-<base-name>)")
	serviceInstallCmd.Flags().String("db", "", "PostgreSQL DSN (if not using --id)")
	serviceInstallCmd.Flags().Int("port", 8080, "HTTP port (if not using --id)")
	serviceInstallCmd.Flags().String("config-source", "database", "file or database (if not using --id)")
	serviceInstallCmd.Flags().String("project", "", "project directory (for file config-source)")
	serviceInstallCmd.Flags().String("user", "", "system user to run the service (Linux only, default: current user)")
	serviceInstallCmd.Flags().Bool("print", false, "print the unit file instead of installing it")

	serviceUninstallCmd.Flags().String("name", "onebase", "service name to remove")

	serviceCmd.AddCommand(serviceInstallCmd, serviceUninstallCmd)
}

// ── install ───────────────────────────────────────────────────────────────────

func runServiceInstall(cmd *cobra.Command, _ []string) error {
	baseID, _ := cmd.Flags().GetString("id")
	svcName, _ := cmd.Flags().GetString("name")
	printOnly, _ := cmd.Flags().GetBool("print")

	var dsn, configSource, project, displayName string
	var port int

	if baseID != "" {
		store, err := launcher.NewStore()
		if err != nil {
			return err
		}
		base, err := store.Get(baseID)
		if err != nil {
			return fmt.Errorf("база не найдена: %w", err)
		}
		dsn = base.DB
		port = base.Port
		configSource = base.ConfigSource
		project = base.Path
		displayName = base.Name
		if svcName == "" {
			svcName = "onebase-" + slugify(base.Name)
		}
	} else {
		dsn, _ = cmd.Flags().GetString("db")
		port, _ = cmd.Flags().GetInt("port")
		configSource, _ = cmd.Flags().GetString("config-source")
		project, _ = cmd.Flags().GetString("project")
		displayName = svcName
		if dsn == "" {
			return fmt.Errorf("укажите --id или --db")
		}
		if svcName == "" {
			svcName = "onebase"
		}
	}

	exe, err := os.Executable()
	if err != nil {
		exe = "onebase"
	}
	exe, _ = filepath.Abs(exe)

	switch runtime.GOOS {
	case "linux":
		return installSystemd(exe, svcName, displayName, dsn, configSource, project, port, cmd, printOnly)
	case "windows":
		return installWindowsService(exe, svcName, displayName, dsn, configSource, project, port, printOnly)
	default:
		return fmt.Errorf("автоустановка сервиса не поддерживается на %s; используйте --print для получения конфигурации", runtime.GOOS)
	}
}

// ── systemd ───────────────────────────────────────────────────────────────────

const systemdUnitTmpl = `[Unit]
Description=OneBase — {{.DisplayName}}
After=network.target postgresql.service
Wants=postgresql.service

[Service]
Type=simple
User={{.User}}
ExecStart={{.Exe}} run --config-source {{.ConfigSource}} --db "{{.DSN}}" --port {{.Port}}{{if .Project}} --project "{{.Project}}"{{end}}
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal
SyslogIdentifier={{.SvcName}}
Environment=HOME={{.Home}}

[Install]
WantedBy=multi-user.target
`

type systemdData struct {
	DisplayName  string
	User         string
	Home         string
	Exe          string
	SvcName      string
	DSN          string
	ConfigSource string
	Project      string
	Port         int
}

func installSystemd(exe, svcName, displayName, dsn, configSource, proj string, port int, cmd *cobra.Command, printOnly bool) error {
	user, _ := cmd.Flags().GetString("user")
	if user == "" {
		user = os.Getenv("USER")
		if user == "" {
			user = "onebase"
		}
	}
	home := os.Getenv("HOME")
	if home == "" {
		home = "/home/" + user
	}

	data := systemdData{
		DisplayName:  displayName,
		User:         user,
		Home:         home,
		Exe:          exe,
		SvcName:      svcName,
		DSN:          dsn,
		ConfigSource: configSource,
		Project:      proj,
		Port:         port,
	}

	tmpl := template.Must(template.New("unit").Parse(systemdUnitTmpl))

	if printOnly {
		return tmpl.Execute(os.Stdout, data)
	}

	unitPath := fmt.Sprintf("/etc/systemd/system/%s.service", svcName)
	f, err := os.Create(unitPath)
	if err != nil {
		return fmt.Errorf("не удалось записать %s (запустите с sudo): %w", unitPath, err)
	}
	defer f.Close()
	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	for _, args := range [][]string{
		{"systemctl", "daemon-reload"},
		{"systemctl", "enable", svcName},
		{"systemctl", "start", svcName},
	} {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "  %s: %s\n", strings.Join(args, " "), out)
		}
	}

	fmt.Printf("Сервис %s установлен и запущен.\n", svcName)
	fmt.Printf("  Статус:  systemctl status %s\n", svcName)
	fmt.Printf("  Логи:    journalctl -u %s -f\n", svcName)
	fmt.Printf("  Стоп:    systemctl stop %s\n", svcName)
	return nil
}

// ── Windows service ───────────────────────────────────────────────────────────

func installWindowsService(exe, svcName, displayName, dsn, configSource, proj string, port int, printOnly bool) error {
	args := fmt.Sprintf(`run --config-source %s --db "%s" --port %d`, configSource, dsn, port)
	if proj != "" {
		args += fmt.Sprintf(` --project "%s"`, proj)
	}

	scCmd := fmt.Sprintf(`sc.exe create "%s" binPath= "%s %s" start= auto DisplayName= "OneBase — %s"`,
		svcName, exe, args, displayName)

	if printOnly {
		fmt.Println("# Выполните от имени администратора:")
		fmt.Println(scCmd)
		fmt.Printf(`sc.exe description "%s" "OneBase business platform"`+"\n", svcName)
		fmt.Printf(`sc.exe start "%s"`+"\n", svcName)
		return nil
	}

	out, err := exec.Command("sc.exe", "create", svcName,
		"binPath=", exe+" "+args,
		"start=", "auto",
		"DisplayName=", "OneBase — "+displayName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("sc.exe create: %w\n%s", err, out)
	}
	exec.Command("sc.exe", "description", svcName, "OneBase business platform").Run()
	exec.Command("sc.exe", "start", svcName).Run()

	fmt.Printf("Сервис %s зарегистрирован в Windows Services.\n", svcName)
	fmt.Printf("  Запуск:  sc.exe start %s\n", svcName)
	fmt.Printf("  Стоп:    sc.exe stop %s\n", svcName)
	fmt.Printf("  Удаление: onebase service uninstall --name %s\n", svcName)
	return nil
}

// ── uninstall ─────────────────────────────────────────────────────────────────

func runServiceUninstall(cmd *cobra.Command, _ []string) error {
	svcName, _ := cmd.Flags().GetString("name")
	switch runtime.GOOS {
	case "linux":
		exec.Command("systemctl", "stop", svcName).Run()
		exec.Command("systemctl", "disable", svcName).Run()
		unitPath := fmt.Sprintf("/etc/systemd/system/%s.service", svcName)
		if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		exec.Command("systemctl", "daemon-reload").Run()
		fmt.Printf("Сервис %s удалён.\n", svcName)
	case "windows":
		exec.Command("sc.exe", "stop", svcName).Run()
		out, err := exec.Command("sc.exe", "delete", svcName).CombinedOutput()
		if err != nil {
			return fmt.Errorf("sc.exe delete: %w\n%s", err, out)
		}
		fmt.Printf("Сервис %s удалён.\n", svcName)
	default:
		return fmt.Errorf("неподдерживаемая ОС: %s", runtime.GOOS)
	}
	return nil
}

func slugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
