package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type SAPClient struct {
	BaseURL    string
	CompanyDB  string
	HTTPClient *http.Client
}

type LoginRequest struct {
	CompanyDB string `json:"CompanyDB"`
	UserName  string `json:"UserName"`
	Password  string `json:"Password"`
}

type DocumentMapping struct {
	Value []struct {
		DocEntry int `json:"DocEntry"`
		DocNum   int `json:"DocNum"`
	} `json:"value"`
}

// TransferRequest representa un documento de solicitud de traslado con los campos que pide la consulta.
type TransferRequest struct {
	DocEntry       int    `json:"DocEntry"`
	DocDate        string `json:"DocDate"`
	DocNum         int    `json:"DocNum"`
	DocumentStatus string `json:"DocumentStatus"`
}

// TransferRequestList es la respuesta de la lista de documentos abiertos.
// ODataNextLink trae la URL de la siguiente página según el dialécto OData:
// en /b1s/v1 la clave es "odata.nextLink" (v3) y en /b1s/v2 "@odata.nextLink" (v4).
type TransferRequestList struct {
	Value            []TransferRequest `json:"value"`
	ODataNextLink    string            `json:"odata.nextLink"`
	ODataNextLinkV4  string            `json:"@odata.nextLink"`
}

func NewSAPClient(baseURL, companyDB string) (*SAPClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	// Inseguro solo para entornos de pruebas sin certificados SSL válidos;
	// configurado con Keep-Alive y Connection Pooling para evitar renegociaciones DNS innecesarias.
	tr := &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}

	client := &http.Client{
		Jar:       jar,
		Transport: tr,
		Timeout:   60 * time.Second,
	}

	return &SAPClient{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		CompanyDB:  companyDB,
		HTTPClient: client,
	}, nil
}

// executeRequest ejecuta una petición HTTP con reintentos automáticos para errores de red o DNS transitorios.
func (s *SAPClient) executeRequest(reqFactory func() (*http.Request, error), op string) (*http.Response, error) {
	const maxRetries = 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := reqFactory()
		if err != nil {
			return nil, NewSAPConnectionError(op, err)
		}

		resp, err := s.HTTPClient.Do(req)
		if err != nil {
			lastErr = NewSAPConnectionError(op, err)
			// Si es un fallo de red / DNS transitorio, esperar y reintentar
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt*400) * time.Millisecond)
				continue
			}
			return nil, lastErr
		}

		return resp, nil
	}

	return nil, lastErr
}

func (s *SAPClient) Login(user, password string) error {
	loginData := LoginRequest{
		CompanyDB: s.CompanyDB,
		UserName:  user,
		Password:  password,
	}

	loginURL := s.BaseURL + "/Login"
	resp, err := s.executeRequest(func() (*http.Request, error) {
		body, _ := json.Marshal(loginData)
		req, err := http.NewRequest(http.MethodPost, loginURL, bytes.NewBuffer(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}, "login")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return NewSAPErrorFromResponse("login", resp.StatusCode, respBody)
	}

	return nil
}

// MapDocNumsToDocEntries obtiene los DocEntry correspondientes a los DocNum abiertos
func (s *SAPClient) MapDocNumsToDocEntries(endpoint string, docNums []string) ([]int, error) {
	if len(docNums) == 0 {
		return nil, nil
	}

	var validNums []string
	for _, dn := range docNums {
		dn = strings.TrimSpace(dn)
		if dn != "" {
			validNums = append(validNums, dn)
		}
	}
	if len(validNums) == 0 {
		return nil, nil
	}

	var allDocEntries []int
	const chunkSize = 30

	for i := 0; i < len(validNums); i += chunkSize {
		end := i + chunkSize
		if end > len(validNums) {
			end = len(validNums)
		}
		chunk := validNums[i:end]

		var orParts []string
		for _, dn := range chunk {
			orParts = append(orParts, fmt.Sprintf("DocNum eq %s", dn))
		}
		filter := fmt.Sprintf("(%s) and DocumentStatus eq 'bost_Open'", strings.Join(orParts, " or "))

		params := url.Values{}
		params.Set("$select", "DocEntry,DocNum")
		params.Set("$filter", filter)
		queryURL := fmt.Sprintf("%s/%s?%s", s.BaseURL, endpoint, params.Encode())

		currentQueryURL := queryURL
		resp, err := s.executeRequest(func() (*http.Request, error) {
			req, err := http.NewRequest(http.MethodGet, currentQueryURL, nil)
			if err != nil {
				return nil, err
			}
			// Service Layer es sensible a estos headers; sin ellos puede devolver 406/415 en algunas versiones.
			req.Header.Set("Accept", "application/json")
			req.Header.Set("Content-Type", "application/json")
			return req, nil
		}, "consulta")
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, NewSAPErrorFromResponseWithURL("consulta", resp.StatusCode, respBody, queryURL)
		}

		var mapping DocumentMapping
		if err := json.NewDecoder(resp.Body).Decode(&mapping); err != nil {
			resp.Body.Close()
			return nil, NewSAPErrorFromResponseWithURL("consulta", resp.StatusCode, []byte("respuesta no es JSON válido: "+err.Error()), queryURL)
		}
		resp.Body.Close()

		for _, doc := range mapping.Value {
			allDocEntries = append(allDocEntries, doc.DocEntry)
		}
	}

	return allDocEntries, nil
}

// GetOpenTransferRequests trae TODOS los documentos abiertos siguiendo el
// nextLink que devuelve Service Layer. SL impone su propio tamaño de página
// en servidor (típico 20) e ignora $top grandes, por eso el page size se
// negocia con el header Prefer: odata.maxpagesize y se sigue el nextLink
// hasta que desaparezca. En /b1s/v2 (OData v4) la clave es @odata.nextLink.
func (s *SAPClient) GetOpenTransferRequests() ([]TransferRequest, error) {
	const maxPageSize = 100
	params := url.Values{}
	params.Set("$select", "DocEntry,DocDate,DocNum,DocumentStatus")
	params.Set("$filter", "DocumentStatus eq 'bost_Open'")
	params.Set("$orderby", "DocNum desc")
	nextURL := fmt.Sprintf("%s/InventoryTransferRequests?%s", s.BaseURL, params.Encode())

	var all []TransferRequest
	seen := map[string]bool{}
	pages := 0

	for nextURL != "" {
		if seen[nextURL] {
			return nil, NewSAPErrorFromResponseWithURL("consulta", 200, []byte("el nextLink se repitió; se cancela para evitar un loop infinito"), nextURL)
		}
		seen[nextURL] = true

		pages++
		if pages > 1000 {
			return nil, NewSAPErrorFromResponseWithURL("consulta", 200, []byte("se excedió el límite de páginas al consultar (posible loop en nextLink)"), nextURL)
		}

		reqURL := nextURL
		resp, err := s.executeRequest(func() (*http.Request, error) {
			req, err := http.NewRequest(http.MethodGet, reqURL, nil)
			if err != nil {
				return nil, err
			}
			// Service Layer puede devolver 406/415 si no se envía Accept explícito.
			req.Header.Set("Accept", "application/json")
			req.Header.Set("Content-Type", "application/json")
			// Negocia el tamaño de página; SL lo responde en el header Preference-Applied.
			req.Header.Set("Prefer", fmt.Sprintf("odata.maxpagesize=%d", maxPageSize))
			return req, nil
		}, "consulta")
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, NewSAPErrorFromResponseWithURL("consulta", resp.StatusCode, respBody, nextURL)
		}

		var list TransferRequestList
		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			resp.Body.Close()
			return nil, NewSAPErrorFromResponseWithURL("consulta", resp.StatusCode, []byte("respuesta no es JSON válido: "+err.Error()), nextURL)
		}
		resp.Body.Close()

		all = append(all, list.Value...)
		next := list.ODataNextLink
		if next == "" {
			next = list.ODataNextLinkV4
		}
		nextURL = resolveNextLink(s.BaseURL, next)
	}

	return all, nil
}

// resolveNextLink convierte el odata.nextLink (relativo o absoluto) en URL absoluta.
func resolveNextLink(baseURL, next string) string {
	next = strings.TrimSpace(strings.Trim(next, `"`))
	if next == "" {
		return ""
	}
	if strings.HasPrefix(next, "http://") || strings.HasPrefix(next, "https://") {
		return next
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(next, "/")
}

// CloseDocument cierra un documento individual haciendo POST a /endpoint(docEntry)/Close
// con reintentos automáticos ante fallos de red o DNS transitorios.
func (s *SAPClient) CloseDocument(endpoint string, docEntry int) error {
	closeURL := fmt.Sprintf("%s/%s(%d)/Close", s.BaseURL, endpoint, docEntry)
	resp, err := s.executeRequest(func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodPost, closeURL, bytes.NewBuffer([]byte("{}")))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}, "cierre")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Service Layer suele devolver 204 No Content en Close exitoso, o 200 OK.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return NewSAPErrorFromResponseWithURL("cierre", resp.StatusCode, respBody, closeURL)
	}

	return nil
}

func (s *SAPClient) CloseDocumentsBatch(endpoint string, docEntries []int) error {
	boundary := "batch_close_boundary"
	changeset := "changeset_close_boundary"

	var body strings.Builder

	body.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	body.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n\r\n", changeset))

	for _, docEntry := range docEntries {
		body.WriteString(fmt.Sprintf("--%s\r\n", changeset))
		body.WriteString("Content-Type: application/http\r\n")
		body.WriteString("Content-Transfer-Encoding: binary\r\n\r\n")
		body.WriteString(fmt.Sprintf("POST %s(%d)/Close HTTP/1.1\r\n\r\n\r\n", endpoint, docEntry))
	}

	body.WriteString(fmt.Sprintf("--%s--\r\n", changeset))
	body.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	req, err := http.NewRequest("POST", s.BaseURL+"/$batch", strings.NewReader(body.String()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", fmt.Sprintf("multipart/mixed; boundary=%s", boundary))

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return NewSAPConnectionError("cierre", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		respBody, _ := io.ReadAll(resp.Body)
		return NewSAPErrorFromResponse("cierre", resp.StatusCode, respBody)
	}

	return nil
}