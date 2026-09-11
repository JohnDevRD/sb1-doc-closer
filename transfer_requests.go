package main

import (
	"fmt"
	"strings"
)

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
			Pause("")
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
	fmt.Println("Consultando documentos abiertos en SAP...")

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
		return
	}

	fmt.Printf("\nSe encontraron %d solicitudes de traslado abiertas:\n\n", len(requests))
	fmt.Println(strings.Repeat("-", 100))
	fmt.Printf("%-8s %-12s %-10s %-15s\n", "DocEntry", "DocNum", "DocDate", "DocumentStatus")
	fmt.Println(strings.Repeat("-", 100))

	for _, req := range requests {
		fmt.Printf("%-8d %-12d %-10s %-15s\n", req.DocEntry, req.DocNum, req.DocDate, req.DocumentStatus)
	}

	fmt.Println(strings.Repeat("-", 100))
	fmt.Printf("\nTotal: %d documentos abiertos\n", len(requests))
}
