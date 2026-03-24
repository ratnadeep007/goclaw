package paths

import (
	"os"
	"path/filepath"
	"runtime"
)

func AppDataDir(app string) string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, app)
	}
	if runtime.GOOS == "windows" {
		if v := os.Getenv("LOCALAPPDATA"); v != "" {
			return filepath.Join(v, app)
		}
		if v := os.Getenv("APPDATA"); v != "" {
			return filepath.Join(v, app)
		}
	}
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".local", "share", app)
	}
	if home := os.Getenv("USERPROFILE"); home != "" {
		return filepath.Join(home, "AppData", "Local", app)
	}
	return filepath.Join(".", app)
}

func ConfigDir(app string) string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, app)
	}
	if runtime.GOOS == "windows" {
		if v := os.Getenv("APPDATA"); v != "" {
			return filepath.Join(v, app)
		}
	}
	if v, err := os.UserConfigDir(); err == nil && v != "" {
		return filepath.Join(v, app)
	}
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".config", app)
	}
	if home := os.Getenv("USERPROFILE"); home != "" {
		return filepath.Join(home, "AppData", "Roaming", app)
	}
	return filepath.Join(".", app)
}
