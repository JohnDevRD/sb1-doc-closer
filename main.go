package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	cfg, _ := LoadConfig()

	ShowHeader()
	for {
		fmt.Println("1. Iniciar Sesión")
		fmt.Println("2. Configuración de Conexión")
		fmt.Println("3. Salir")
		fmt.Print("Seleccione una opción: ")

		var opc string
		fmt.Scanln(&opc)

		switch opc {
		case "1":
			if cfg.ServiceLayerURL == "" || cfg.CompanyDB == "" {
				fmt.Println("\n[!] Debe configurar la URL y la Base de Datos antes de iniciar sesión.")
				Pause("")
				cfg = ConfigurePrompt(cfg)
				ShowHeader()
				continue
			}
			runApp(cfg)
			ShowHeader()
		case "2":
			cfg = ConfigurePrompt(cfg)
			ShowHeader()
		case "3":
			ClearScreen()
			fmt.Println("Saliendo...")
			return
		default:
			fmt.Println("Opción no válida.")
			Pause("")
			ShowHeader()
		}
	}
}

func runApp(cfg Config) {
	reader := bufio.NewReader(os.Stdin)

	ClearScreen()
	fmt.Println("=== INICIO DE SESIÓN ===")
	fmt.Print("Usuario SAP: ")
	user, _ := reader.ReadString('\n')
	user = strings.TrimSpace(user)

	pass := ReadPassword("Contraseña SAP: ")
	if pass == "" {
		fmt.Println("\n[!] La contraseña no puede estar vacía.")
		Pause("")
		return
	}

	client, err := NewSAPClient(cfg.ServiceLayerURL, cfg.CompanyDB)
	if err != nil {
		fmt.Printf("Error al inicializar cliente HTTP: %v\n", err)
		Pause("")
		return
	}

	fmt.Println("Conectando con Service Layer...")
	if err := client.Login(user, pass); err != nil {
		fmt.Printf("\n[Error]: %s\n", FriendlyErrorMessage(err, "No se pudo iniciar sesión. Verifique sus credenciales."))
		Pause("")
		return
	}

	fmt.Println("\n¡Sesión iniciada con éxito!")
	Pause("")

	for {
		ShowDocMenuHeader()
		fmt.Println("1. Solicitudes de traslado")
		fmt.Println("2. Órdenes de compras (Próximamente)")
		fmt.Println("3. Ofertas de ventas (Próximamente)")
		fmt.Println("4. Solicitudes de devolución (Próximamente)")
		fmt.Println("5. Solicitudes de compras (Próximamente)")
		fmt.Println("6. Entradas de mercancía (Próximamente)")
		fmt.Println("7. Devolución de mercancías (Próximamente)")
		fmt.Println("8. Recuento de inventario (Próximamente)")
		fmt.Println("9. Cerrar Sesión / Volver")
		fmt.Print("Seleccione el documento: ")

		var docOpc string
		fmt.Scanln(&docOpc)

		switch docOpc {
		case "1":
			processCloseDocument(client, "InventoryTransferRequests", "Solicitudes de Traslado")
			Pause("")
		case "9":
			return
		default:
			fmt.Println("Opción no implementada o no válida por el momento.")
			Pause("")
		}
	}
}

func processCloseDocument(client *SAPClient, endpoint, docName string) {
	ClearScreen()
	fmt.Printf("--- CIERRE MASIVO DE: %s ---\n", strings.ToUpper(docName))
	fmt.Println("Ingrese o pegue los Numeros de Documentos separados por comas o espacios:")
	
	input := ReadLine()

	if input == "" {
		fmt.Println("No ingresó números de documento.")
		return
	}

	// Formatear la entrada eliminando comas sobrantes y dividiendo por espacios/comas
	rawList := strings.Fields(strings.ReplaceAll(input, ",", " "))
	if len(rawList) == 0 {
		fmt.Println("Lista de documentos vacía.")
		return
	}

	fmt.Printf("Buscando %d documentos abiertos en SAP...\n", len(rawList))
	docEntries, err := client.MapDocNumsToDocEntries(endpoint, rawList)
	if err != nil {
		fmt.Printf("Error obteniendo datos: %s\n", FriendlyErrorMessage(err, "No se pudieron obtener los documentos. Intente de nuevo."))
		return
	}

	if len(docEntries) == 0 {
		fmt.Println("No se encontraron documentos abiertos para los DocNum ingresados.")
		return
	}

	fmt.Printf("Se encontraron %d documentos listos para cerrar. Enviando petición masiva ($batch)...\n", len(docEntries))
	if err := client.CloseDocumentsBatch(endpoint, docEntries); err != nil {
		fmt.Printf("Error ejecutando el cierre masivo: %s\n", FriendlyErrorMessage(err, "No se pudo completar el cierre masivo. Intente de nuevo."))
	} else {
		fmt.Println("¡Proceso de cierre enviado exitosamente!")
	}
}