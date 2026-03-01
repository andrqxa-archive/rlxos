package svg

import (
	"path/filepath"
	"testing"
)

func BenchmarkDecodeFileClock(b *testing.B) {
	path := filepath.Join("..", "..", "data", "icons", "default", "clock.svg")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf, err := DecodeFile(path, 64)
		if err != nil {
			b.Fatal(err)
		}
		if buf == nil || buf.Width <= 0 || buf.Height <= 0 {
			b.Fatal("invalid buffer")
		}
	}
}

func BenchmarkDecodeFileIconSet64(b *testing.B) {
	icons := []string{
		"notes.svg",
		"arrow_up.svg",
		"taskmanager.svg",
		"star.svg",
		"notepad.svg",
		"filemanager.svg",
		"image.svg",
		"oobe.svg",
		"terminal.svg",
		"background.svg",
		"monitor.svg",
		"lock_closed.svg",
		"dock.svg",
		"clock.svg",
		"monitor_analytics.svg",
		"power.svg",
		"folder.svg",
		"apps_grid.svg",
		"demo.svg",
		"help.svg",
	}
	paths := make([]string, 0, len(icons))
	for _, icon := range icons {
		paths = append(paths, filepath.Join("..", "..", "data", "icons", "default", icon))
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, p := range paths {
			buf, err := DecodeFile(p, 64)
			if err != nil {
				b.Fatal(err)
			}
			if buf == nil || buf.Width <= 0 || buf.Height <= 0 {
				b.Fatal("invalid buffer")
			}
		}
	}
}
