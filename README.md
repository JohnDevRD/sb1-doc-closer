# SAP B1 - Cierre Masivo (Service Layer)

CLI en Go para cerrar documentos abiertos de SAP Business One vía **Service Layer** (HTTP/SOAP REST).

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
3. Selecciona el tipo de documento a cerrar y pasa los números de documento (`DocNum`) separados por comas o espacios.

Ejemplo de cierre masivo:

```
Ingrese o pegue los Numeros de Documentos separados por comas o espacios:
12345, 12346, 12347
```

### Documentos soportados

- Solicitudes de traslado (InventoryTransferRequests) — disponible
- Órdenes de compras — próximamente
- (y demás documentos marcados como próximamente en el menú)

## Estructura del proyecto

- `main.go` — menú principal y flujo de la aplicación
- `sap_client.go` — cliente HTTP contra Service Layer (login, consulta y cierre)
- `config.go` — carga/guardado de `config.json`
- `ui.go` — utilidades de terminal (limpiar pantalla, ocultar password, etc.)
- `errors.go` — manejo de errores SAP con mensajes amigables

## Seguridad

El cliente usa conexión TLS. En algunos entornos de prueba sin certificados válidos, el código **omite la verificación TLS** (`InsecureSkipVerify: true`). Esto está pensado solo para entornos de desarrollo/prueba controlados.

Para entornos de producción es recomendable usar certificados válidos y quitar `InsecureSkipVerify` en `sap_client.go`.

## Licencia

MIT
