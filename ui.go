package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"golang.org/x/term"
)

// ClearScreen limpia la ventana de la terminal de forma multiplataforma.
// En Windows usa `cls`, en Linux/macOS usa `clear`, y como respaldo usa secuencias ANSI.
func ClearScreen() {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		_ = cmd.Run()
	} else {
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		_ = cmd.Run()
	}
	// Respaldo por si el comando anterior no limpió (ej. terminal integrada):
	fmt.Print("\033[H\033[2J")
}

// Pause muestra un mensaje y espera ENTER para que el usuario
// alcance a leer antes de que la pantalla se limpie.
func Pause(msg string) {
	if msg == "" {
		msg = "\nPresione ENTER para continuar..."
	}
	fmt.Print(msg)
	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadString('\n')
}

// ShowHeader limpia y dibuja el encabezado principal.
func ShowHeader() {
	ClearScreen()
	fmt.Println("====================================")
	fmt.Println("  SAP B1 - CIERRE MASIVO (SERVICE LAYER)")
	fmt.Println("====================================")
}

// ShowDocMenuHeader limpia y dibuja el encabezado del menú de documentos.
func ShowDocMenuHeader() {
	ClearScreen()
	fmt.Println("====================================")
	fmt.Println("  SAP B1 - CIERRE MASIVO (SERVICE LAYER)")
	fmt.Println("====================================")
	fmt.Println("--- MENÚ DE DOCUMENTOS A CERRAR ---")
}

// ReadLine lee una línea completa de stdin (incluye espacios) y la limpia.
func ReadLine() string {
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

// ReadPassword pide una contraseña sin mostrarla en pantalla (sin eco).
// Funciona en Windows, Linux y macOS gracias a golang.org/x/term.
// Si stdin no es una terminal (ej. pipe), hace fallback a lectura normal.
func ReadPassword(prompt string) string {
	fmt.Print(prompt)
	bytePass, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println() // salto de línea tras el ENTER (ReadPassword no lo imprime)
	if err != nil {
		// Fallback: stdin no es terminal o hubo error -> leer visible
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		return strings.TrimSpace(line)
	}
	return strings.TrimSpace(string(bytePass))
}
