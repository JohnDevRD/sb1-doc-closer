# SAP B1 - Cierre de Documentos (Service Layer)

CLI en Go para cerrar documentos abiertos de SAP Business One vía **Service Layer** (REST). El cierre se realiza documento por documento mediante `POST /<EndPoint>(DocEntry)/Close`, mostrando progreso y un resumen final de éxitos/errores.

## Requisitos

- [Go](https://go.dev/) 1.26.4 (o superior si lo soporta)
- Acceso a un servidor SAP Business One con **Service Layer** habilitado

## Instalación

```bash
go build -o sb1-doc-closer
```

Esto genera el ejecutable `sb1-doc-closer` (o `sb1-doc-closer.exe` en Windows).

## Configuración

Edita `config.json` con los datos de tu entorno:

```json
{
  "service_layer_url": "https://TU_SERVER:50000/b1s/v2",
  "company_db": "TU_COMPANYDB"
}
```

## Uso

```bash
./sb1-doc-closer
```

### Flujo básico

1. Si no está configurado, puedes usarlo en modo **Configuración de Conexión** desde el menú principal.
2. Usa **Iniciar Sesión** para autenticarte contra SAP.
3. Selecciona el tipo de documento y sigue el flujo indicado en el menú.

**Solicitudes de traslado**:
1. Selecciona **Consultar documentos abiertos** para listar las solicitudes abiertas (paginas de 25).
2. Marca/desmarca registros con los comandos de la pantalla (`[1-25]`, `[P]ágina completa`, `[T]odos`, `[L]impiar`, `[V]er sel.`).
3. Confirma con `[C]errar sel.`; cada documento se cierra individualmente mostrando `[n/N]` y un resumen final.

Ejemplo del proceso de cierre:

```
[1/3] Cerrando DocEntry 1042... OK (Cerrado)
[2/3] Cerrando DocEntry 1045... ERROR: El documento ya está cerrado...
[3/3] Cerrando DocEntry 1050... OK (Cerrado)

Resumen: 2 documento(s) cerrado(s) exitosamente, 1 con error.
```

## Depuración

Si ocurre un error, la herramienta muestra un mensaje amigable. Para ver el detalle técnico (URL y respuesta cruda de SAP), ejecuta con la variable `SAP_DEBUG=1`:

```bash
# PowerShell
$env:SAP_DEBUG="1"; ./sb1-doc-closer

# Linux/macOS
SAP_DEBUG=1 ./sb1-doc-closer
```

### Documentos soportados

- Solicitudes de traslado (InventoryTransferRequests) — disponible
- Órdenes de compras — próximamente
- (y demás documentos marcados como próximamente en el menú)

## Estructura del proyecto

- `main.go` — menú principal y flujo de la aplicación
- `sap_client.go` — cliente HTTP contra Service Layer (login, consulta por rangos y cierre por documento)
- `transfer_requests.go` — listado paginado y selección de solicitudes de traslado
- `config.go` — carga/guardado de `config.json`
- `ui.go` — utilidades de terminal (limpiar pantalla, ocultar password, etc.)
- `errors.go` — manejo de errores SAP con mensajes amigables y detalle vía `SAP_DEBUG`

## Seguridad

El cliente usa conexión TLS. En algunos entornos de prueba sin certificados válidos, el código **omite la verificación TLS** (`InsecureSkipVerify: true`). Esto está pensado solo para entornos de desarrollo/prueba controlados.

Para entornos de producción es recomendable usar certificados válidos y quitar `InsecureSkipVerify` en `sap_client.go`.

## Licencia

MIT
