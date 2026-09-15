package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// pageSize cuántos documentos se muestran por página.
const pageSize = 25

func showTransferRequestMenu(client *SAPClient) {
	for {
		ClearScreen()
		fmt.Println("--- SOLICITUDES DE TRASLADO ---")
		fmt.Println("1. Consultar documentos abiertos")
		fmt.Println("2. Volver")
		fmt.Print("Seleccione una opción: ")

		var opc string
		fmt.Scanln(&opc)

		switch opc {
		case "1":
			listOpenTransferRequests(client)
		case "2":
			return
		default:
			fmt.Println("Opción no válida.")
			Pause("")
		}
	}
}

func listOpenTransferRequests(client *SAPClient) {
	ClearScreen()
	fmt.Println("=== SOLICITUDES DE TRASLADO ABIERTAS ===")
	fmt.Println("Consultando documentos abiertos en SAP (puede tardar, trayendo todos)...")

	requests, err := client.GetOpenTransferRequests()
	if err != nil {
		fmt.Printf("Error al obtener los documentos: %s\n", FriendlyErrorMessage(err, "No se pudieron obtener los documentos. Intente de nuevo."))
		if d := DebugDetail(err); d != "" {
			fmt.Println(d)
		} else {
			fmt.Println("(Tip: ejecute con SAP_DEBUG=1 para ver el detalle técnico del error.)")
		}
		return
	}

	if len(requests) == 0 {
		fmt.Println("No se encontraron solicitudes de traslado abiertas.")
		Pause("")
		return
	}

	browseAndSelectTransferRequests(client, requests)
}

// browseAndSelectTransferRequests muestra los documentos paginados de 25 y
// permite seleccionar registros, con cierre masivo al confirmar.
func browseAndSelectTransferRequests(client *SAPClient, requests []TransferRequest) {
	totalPages := (len(requests) + pageSize - 1) / pageSize
	page := 0
	// selected guarda índices globales (0-based) marcados con [X].
	selected := map[int]bool{}

	for {
		ClearScreen()
		fmt.Printf("=== SOLICITUDES DE TRASLADO ABIERTAS (%d) ===\n", len(requests))
		fmt.Printf("Página %d de %d | Seleccionados: %d\n\n", page+1, totalPages, len(selected))

		start := page * pageSize
		end := start + pageSize
		if end > len(requests) {
			end = len(requests)
		}

		fmt.Println(strings.Repeat("-", 80))
		fmt.Printf("%-4s %-4s %-12s %-12s %-15s\n", "#", "Sel", "DocNum", "Fecha", "Estado")
		fmt.Println(strings.Repeat("-", 80))

		for i := start; i < end; i++ {
			mark := "[ ]"
			if selected[i] {
				mark = "[X]"
			}
			// # es el número visible en esta página (1-25) para seleccionar rápido.
			fmt.Printf("%-4d %-4s %-12d %-12s %-15s\n",
				i-start+1, mark, requests[i].DocNum, shortDate(requests[i].DocDate), friendlyStatus(requests[i].DocumentStatus))
		}

		fmt.Println(strings.Repeat("-", 80))
		fmt.Println("Comandos: [N]sig [A]nt [1-25] marcar/desmarcar [P]ágina completa [T]odos [L]impiar [V]er sel. [C]errar sel. [S]alir")
		fmt.Print("Opción: ")

		raw := strings.TrimSpace(ReadLine())
		upper := strings.ToUpper(raw)

		switch upper {
		case "N":
			if page < totalPages-1 {
				page++
			} else {
				fmt.Println("Ya está en la última página.")
				Pause("")
			}
		case "A":
			if page > 0 {
				page--
			} else {
				fmt.Println("Ya está en la primera página.")
				Pause("")
			}
		case "P":
			// Marcar/desmarcar toda la página actual.
			allMarked := true
			for i := start; i < end; i++ {
				if !selected[i] {
					allMarked = false
					break
				}
			}
			for i := start; i < end; i++ {
				if allMarked {
					delete(selected, i)
				} else {
					selected[i] = true
				}
			}
		case "T":
			// Marcar/desmarcar TODOS los registros.
			if len(selected) == len(requests) {
				selected = map[int]bool{}
				fmt.Println("Selección de todos los registros limpiada.")
			} else {
				for i := range requests {
					selected[i] = true
				}
				fmt.Printf("Se seleccionaron los %d registros.\n", len(requests))
			}
			Pause("")
		case "L":
			selected = map[int]bool{}
			fmt.Println("Selección limpiada.")
			Pause("")
		case "V":
			showSelectedTransferRequests(requests, selected)
		case "C":
			if len(selected) == 0 {
				fmt.Println("No hay registros seleccionados.")
				Pause("")
				continue
			}
			if confirmCloseTransferRequests(requests, selected) {
				closeSelectedTransferRequests(client, requests, selected)
				return
			}
		case "S":
			return
		default:
			// Selección por números: "1,3,5-8" (números visibles de la página actual).
			idxs, ok := parsePageSelection(raw, end-start)
			if !ok {
				fmt.Println("Opción no válida.")
				Pause("")
				continue
			}
			for _, n := range idxs {
				global := start + (n - 1)
				if selected[global] {
					delete(selected, global)
				} else {
					selected[global] = true
				}
			}
		}
	}
}

// showSelectedTransferRequests muestra el resumen de lo marcado.
func showSelectedTransferRequests(requests []TransferRequest, selected map[int]bool) {
	ClearScreen()
	if len(selected) == 0 {
		fmt.Println("No hay registros seleccionados.")
		Pause("")
		return
	}
	idxs := sortedKeys(selected)
	fmt.Printf("=== REGISTROS SELECCIONADOS (%d) ===\n\n", len(idxs))
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("%-12s %-12s\n", "DocNum", "Fecha")
	fmt.Println(strings.Repeat("-", 60))
	for _, i := range idxs {
		fmt.Printf("%-12d %-12s\n", requests[i].DocNum, shortDate(requests[i].DocDate))
	}
	fmt.Println(strings.Repeat("-", 60))
	Pause("")
}

// confirmCloseTransferRequests pide confirmación antes del cierre masivo.
func confirmCloseTransferRequests(requests []TransferRequest, selected map[int]bool) bool {
	ClearScreen()
	idxs := sortedKeys(selected)
	fmt.Printf("Va a CERRAR %d documento(s):\n\n", len(idxs))
	preview := idxs
	if len(preview) > 15 {
		preview = preview[:15]
	}
	for _, i := range preview {
		fmt.Printf("  - DocNum %d (%s)\n", requests[i].DocNum, shortDate(requests[i].DocDate))
	}
	if len(idxs) > len(preview) {
		fmt.Printf("  ... y %d más\n", len(idxs)-len(preview))
	}
	fmt.Print("\n¿Confirmar cierre? [S/N]: ")
	ans := strings.ToUpper(strings.TrimSpace(ReadLine()))
	return ans == "S" || ans == "SI" || ans == "SÍ" || ans == "Y" || ans == "YES"
}

// closeSelectedTransferRequests cierra cada documento seleccionado haciendo POST a InventoryTransferRequests(DocEntry)/Close.
func closeSelectedTransferRequests(client *SAPClient, requests []TransferRequest, selected map[int]bool) {
	ClearScreen()
	idxs := sortedKeys(selected)

	if len(idxs) == 0 {
		fmt.Println("No hay registros seleccionados para cerrar.")
		Pause("")
		return
	}

	fmt.Printf("Iniciando cierre de %d documento(s) en SAP...\n\n", len(idxs))

	successCount := 0
	failCount := 0

	for n, i := range idxs {
		doc := requests[i]
		fmt.Printf("[%d/%d] Cerrando DocNum %d (DocEntry %d)... ", n+1, len(idxs), doc.DocNum, doc.DocEntry)

		err := client.CloseDocument("InventoryTransferRequests", doc.DocEntry)
		if err != nil {
			failCount++
			fmt.Printf("ERROR: %s\n", FriendlyErrorMessage(err, "No se pudo cerrar."))
			if d := DebugDetail(err); d != "" {
				fmt.Println(d)
			}
		} else {
			successCount++
			fmt.Println("OK (Cerrado)")
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("Resumen: %d documento(s) cerrado(s) exitosamente, %d con error.\n", successCount, failCount)
	fmt.Println(strings.Repeat("-", 60))
	Pause("")
}

// parsePageSelection interpreta "1,3,5-8" en números de fila de la página (1-based).
func parsePageSelection(raw string, pageCount int) (nums []int, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}
	seen := map[int]bool{}
	parts := strings.Fields(strings.ReplaceAll(raw, ",", " "))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.Contains(p, "-") {
			b := strings.SplitN(p, "-", 2)
			a, err1 := strconv.Atoi(strings.TrimSpace(b[0]))
			c, err2 := strconv.Atoi(strings.TrimSpace(b[1]))
			if err1 != nil || err2 != nil || a < 1 || c < 1 || a > pageCount || c > pageCount || c < a {
				return nil, false
			}
			for n := a; n <= c; n++ {
				if !seen[n] {
					seen[n] = true
					nums = append(nums, n)
				}
			}
		} else {
			n, err := strconv.Atoi(p)
			if err != nil || n < 1 || n > pageCount {
				return nil, false
			}
			if !seen[n] {
				seen[n] = true
				nums = append(nums, n)
			}
		}
	}
	if len(nums) == 0 {
		return nil, false
	}
	sort.Ints(nums)
	return nums, true
}

func sortedKeys(m map[int]bool) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

// shortDate convierte "2026-09-02T00:00:00Z" en "2026-09-02".
func shortDate(s string) string {
	if i := strings.Index(s, "T"); i > 0 {
		return s[:i]
	}
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// friendlyStatus traduce "bost_Open" a "Abierto".
func friendlyStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "bost_open", "open", "o", "abierto":
		return "Abierto"
	case "bost_close", "closed", "c", "cerrado":
		return "Cerrado"
	default:
		return s
	}
}

