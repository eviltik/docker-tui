package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"time"
)

const crashLogPath = "/tmp/docker-tui-crash.log"

// writeCrashLog écrit un crash log détaillé dans /tmp/docker-tui-crash.log
func writeCrashLog(r interface{}, goroutineName string) {
	if r == nil {
		return
	}

	// Ouvrir le fichier en mode append
	f, err := os.OpenFile(crashLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		// Fallback sur stderr si impossible d'ouvrir le fichier
		fmt.Fprintf(os.Stderr, "Failed to open crash log: %v\n", err)
		f = os.Stderr
	}
	defer f.Close()

	// Header avec timestamp
	fmt.Fprintf(f, "\n\n")
	fmt.Fprintf(f, "═══════════════════════════════════════════════════════════════\n")
	fmt.Fprintf(f, "CRASH REPORT - %s\n", time.Now().Format("2006-01-02 15:04:05.000"))
	fmt.Fprintf(f, "═══════════════════════════════════════════════════════════════\n\n")

	// Goroutine qui a crashé
	if goroutineName != "" {
		fmt.Fprintf(f, "Goroutine: %s\n\n", goroutineName)
	} else {
		fmt.Fprintf(f, "Goroutine: main\n\n")
	}

	// Erreur
	fmt.Fprintf(f, "Error: %v\n\n", r)

	// Stacktrace de la goroutine qui a crashé
	fmt.Fprintf(f, "Crashing Goroutine Stack Trace:\n")
	fmt.Fprintf(f, "───────────────────────────────────────────────────────────────\n")
	f.Write(debug.Stack())
	fmt.Fprintf(f, "\n")

	// Stacktraces de TOUTES les goroutines (si GOTRACEBACK=all)
	fmt.Fprintf(f, "All Goroutines Stack Dump:\n")
	fmt.Fprintf(f, "───────────────────────────────────────────────────────────────\n")
	buf := make([]byte, 1024*1024) // 1MB buffer for all goroutines
	stackLen := runtime.Stack(buf, true)
	f.Write(buf[:stackLen])
	fmt.Fprintf(f, "\n")

	// Informations système
	fmt.Fprintf(f, "System Information:\n")
	fmt.Fprintf(f, "───────────────────────────────────────────────────────────────\n")

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fmt.Fprintf(f, "Goroutines:        %d\n", runtime.NumGoroutine())
	fmt.Fprintf(f, "Memory Allocated:  %d MB\n", m.Alloc/1024/1024)
	fmt.Fprintf(f, "Memory Total:      %d MB\n", m.TotalAlloc/1024/1024)
	fmt.Fprintf(f, "Memory Sys:        %d MB\n", m.Sys/1024/1024)
	fmt.Fprintf(f, "GC Runs:           %d\n", m.NumGC)
	fmt.Fprintf(f, "File Descriptors:  %d\n", countOpenFDs())

	fmt.Fprintf(f, "\n")
	fmt.Fprintf(f, "═══════════════════════════════════════════════════════════════\n\n")

	// Aussi afficher sur stderr (même si défoncé par bubbletea)
	if f != os.Stderr {
		fmt.Fprintf(os.Stderr, "\n\n\033[31m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n")
		fmt.Fprintf(os.Stderr, "\033[31m✗ Fatal Error\033[0m\n\n")
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", r)
		fmt.Fprintf(os.Stderr, "\033[33mFull crash log saved to: %s\033[0m\n", crashLogPath)
		fmt.Fprintf(os.Stderr, "\033[31m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m\n\n")
		fmt.Fprintf(os.Stderr, "Please report this issue at: https://github.com/eviltik/docker-tui/issues\n\n")
	}
}

// safeGo lance une goroutine avec protection panic automatique
// Le nom permet d'identifier quelle goroutine a crashé dans les logs
func safeGo(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				writeCrashLog(r, name)
				// Ne pas os.Exit() ici, laisser le programme continuer si possible
			}
		}()
		fn()
	}()
}
