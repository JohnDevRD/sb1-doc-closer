package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// DebugEnabled indica si se debe mostrar detalle técnico.
// Actívelo con: $env:SAP_DEBUG="1" (PowerShell) o SAP_DEBUG=1 go run .
func DebugEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("SAP_DEBUG")))
	return v == "1" || v == "true" || v == "yes"
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncado)"
}

// SAPError representa un error de Service Layer ya traducido
// a un mensaje amigable para el usuario final (sin JSON crudo).
type SAPError struct {
	Op         string // "login", "consulta", "cierre", "conexion"
	StatusCode int    // 0 = error de red / no hubo respuesta HTTP
	SAPCode    string // code que devuelve SAP en el JSON (ej. "-304")
	SAPMessage string // message que devuelve SAP en el JSON
	RawBody    string // cuerpo crudo (solo se muestra con SAP_DEBUG=1)
	URL        string // URL consultada (solo se muestra con SAP_DEBUG=1)
	Err        error  // error original de red (si StatusCode == 0)
}

// sapErrorPayload es la forma del JSON de error de Service Layer:
// {"error":{"code":"-304","message":"..."}}
type sapErrorPayload struct {
	Error struct {
		Code    interface{} `json:"code"`
		Message string      `json:"message"`
	} `json:"error"`
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}

// NewSAPErrorFromResponse construye un SAPError a partir del status HTTP y el body.
func NewSAPErrorFromResponse(op string, statusCode int, body []byte) *SAPError {
	return NewSAPErrorFromResponseWithURL(op, statusCode, body, "")
}

// NewSAPErrorFromResponseWithURL igual que NewSAPErrorFromResponse pero guarda la URL para debug.
func NewSAPErrorFromResponseWithURL(op string, statusCode int, body []byte, url string) *SAPError {
	e := &SAPError{Op: op, StatusCode: statusCode, URL: url, RawBody: truncate(strings.TrimSpace(string(body)), 2000)}
	var p sapErrorPayload
	if len(body) > 0 {
		_ = json.Unmarshal(body, &p)
		e.SAPCode = toString(p.Error.Code)
		e.SAPMessage = strings.TrimSpace(p.Error.Message)
	}
	return e
}

// NewSAPConnectionError construye un SAPError para fallos de red (sin respuesta HTTP).
func NewSAPConnectionError(op string, err error) *SAPError {
	return &SAPError{Op: op, StatusCode: 0, Err: err}
}

// Error implementa error. Devuelve SIEMPRE el mensaje amigable,
// nunca el JSON crudo, para que un simple "%v" sea seguro en la UI.
func (e *SAPError) Error() string {
	return e.FriendlyMessage()
}

// FriendlyMessage traduce el error técnico a texto para el usuario final.
func (e *SAPError) FriendlyMessage() string {
	if e == nil {
		return "Ocurrió un error inesperado."
	}

	// Sin respuesta HTTP: problema de red / URL / SAP apagado.
	if e.StatusCode == 0 {
		msg := ""
		if e.Err != nil {
			msg = strings.ToLower(e.Err.Error())
		}
		switch {
		case strings.Contains(msg, "no such host"),
			strings.Contains(msg, "cannot resolve"),
			strings.Contains(msg, "dns"):
			return "No se encontró el servidor. Verifique la URL del Service Layer en Configuración de Conexión."
		case strings.Contains(msg, "connection refused"),
			strings.Contains(msg, "no connection could be made"),
			strings.Contains(msg, "unreachable"),
			strings.Contains(msg, "timeout"),
			strings.Contains(msg, "deadline exceeded"),
			strings.Contains(msg, "connection reset"):
			return "No se pudo conectar con Service Layer. Verifique que la URL sea correcta, tenga conexión de red y que SAP Business One esté disponible."
		case strings.Contains(msg, "certificate"),
			strings.Contains(msg, "tls"),
			strings.Contains(msg, "ssl"):
			return "No se pudo establecer una conexión segura con el servidor. Contacte al administrador del sistema."
		default:
			return "No se pudo conectar con Service Layer. Verifique la URL configurada, su conexión de red y que SAP esté disponible."
		}
	}

	lowerSAP := strings.ToLower(e.SAPMessage)

	switch e.Op {
	case "login":
		// Credenciales incorrectas: 401 y el clásico -304 / NONE-SSO de SAP.
		if e.StatusCode == httpStatusUnauthorized() ||
			e.SAPCode == "-304" ||
			strings.Contains(lowerSAP, "none-sso") ||
			strings.Contains(lowerSAP, "invalid user") ||
			strings.Contains(lowerSAP, "wrong user") ||
			strings.Contains(lowerSAP, "password") ||
			strings.Contains(lowerSAP, "fail to") && strings.Contains(lowerSAP, "login") {
			return "Usuario o contraseña incorrectos. Verifique sus credenciales e intente de nuevo."
		}
		if e.StatusCode == 400 || e.StatusCode == 404 {
			if strings.Contains(lowerSAP, "company") || strings.Contains(lowerSAP, "database") || strings.Contains(lowerSAP, "db") {
				return "No se pudo conectar a la base de datos de la compañía. Verifique la configuración de conexión."
			}
			return "No se pudo iniciar sesión. Verifique la URL del Service Layer y la base de datos en Configuración de Conexión."
		}
		if e.StatusCode == 403 {
			return "Acceso denegado. El usuario no tiene permiso para iniciar sesión en Service Layer."
		}
		if e.StatusCode >= 500 {
			return "El servidor SAP devolvió un error. Intente de nuevo más tarde y si persiste contacte al administrador."
		}
		return "No se pudo iniciar sesión. Verifique sus credenciales y la configuración de conexión."
	case "consulta":
		if e.StatusCode == 401 || e.StatusCode == 403 {
			return "La sesión expiró o no tiene permisos. Vuelva a iniciar sesión."
		}
		if e.StatusCode == 400 {
			if strings.Contains(lowerSAP, "invalid") && strings.Contains(lowerSAP, "filter") ||
				strings.Contains(lowerSAP, "query") || strings.Contains(lowerSAP, "syntax") {
				return "SAP rechazó la consulta (filtro inválido). Verifique la versión del Service Layer."
			}
			return "SAP rechazó la consulta. Verifique la configuración de conexión."
		}
		if e.StatusCode == 404 {
			return "No se encontró el recurso en SAP. Verifique la URL del Service Layer (debe terminar en /b1s/v2) y la configuración."
		}
		if e.StatusCode == 405 || e.StatusCode == 501 {
			return "Operación no soportada por esta versión de Service Layer."
		}
		if e.StatusCode >= 500 {
			return "El servidor SAP devolvió un error al consultar. Intente de nuevo más tarde."
		}
		// 400 u otros con mensaje genérico: si SAP no dio message útil, dar pista de permisos/licencia
		if e.SAPMessage == "" {
			return "No se pudieron obtener los documentos. Es posible que el usuario no tenga permiso sobre Solicitudes de traslado o que la sesión haya expirado. Vuelva a iniciar sesión."
		}
		return "No se pudieron obtener los documentos. Intente de nuevo."
	case "cierre":
		if e.StatusCode == 401 || e.StatusCode == 403 {
			return "La sesión expiró o no tiene permisos para cerrar documentos. Vuelva a iniciar sesión."
		}
		if e.StatusCode >= 500 {
			return "El servidor SAP devolvió un error durante el cierre. Verifique en SAP qué documentos quedaron cerrados e intente de nuevo."
		}
		return "No se pudo completar el cierre masivo. Intente de nuevo."
	default:
		return "Ocurrió un error de comunicación con SAP. Intente de nuevo."
	}
}

// FriendlyErrorMessage devuelve el mensaje amigable de cualquier error.
// Si es *SAPError usa su traducción; si no, devuelve un texto genérico
// (evita exponer detalles técnicos al usuario).
func FriendlyErrorMessage(err error, fallback string) string {
	if err == nil {
		return ""
	}
	if se, ok := err.(*SAPError); ok {
		return se.FriendlyMessage()
	}
	if fallback == "" {
		fallback = "Ocurrió un error inesperado. Intente de nuevo."
	}
	return fallback
}

// DebugDetail devuelve el detalle técnico (status, code, URL, body)
// SOLO cuando SAP_DEBUG=1. Devuelve "" en modo normal para no exponer JSON.
func DebugDetail(err error) string {
	se, ok := err.(*SAPError)
	if !ok || se == nil {
		if err != nil && DebugEnabled() {
			return fmt.Sprintf("[debug] error técnico: %v", err)
		}
		return ""
	}
	if !DebugEnabled() {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[debug] status=%d", se.StatusCode)
	if se.SAPCode != "" {
		fmt.Fprintf(&b, " sapCode=%s", se.SAPCode)
	}
	if se.SAPMessage != "" {
		fmt.Fprintf(&b, " sapMessage=%s", truncate(se.SAPMessage, 500))
	}
	if se.URL != "" {
		fmt.Fprintf(&b, "\n[debug] url=%s", se.URL)
	}
	if se.RawBody != "" {
		fmt.Fprintf(&b, "\n[debug] respuesta=%s", se.RawBody)
	}
	if se.Err != nil {
		fmt.Fprintf(&b, "\n[debug] red=%v", se.Err)
	}
	return b.String()
}

// httpStatusUnauthorized evita importar net/http solo por la constante.
func httpStatusUnauthorized() int { return 401 }
