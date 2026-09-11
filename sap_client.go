package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
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
type TransferRequestList struct {
	Value []TransferRequest `json:"value"`
}

func NewSAPClient(baseURL, companyDB string) (*SAPClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	// Inseguro solo para entornos de pruebas sin certificados SSL válidos
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{
		Jar:       jar,
		Transport: tr,
	}

	return &SAPClient{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		CompanyDB:  companyDB,
		HTTPClient: client,
	}, nil
}

func (s *SAPClient) Login(user, password string) error {
	loginData := LoginRequest{
		CompanyDB: s.CompanyDB,
		UserName:  user,
		Password:  password,
	}

	body, _ := json.Marshal(loginData)
	resp, err := s.HTTPClient.Post(s.BaseURL+"/Login", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return NewSAPConnectionError("login", err)
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
	filterNums := strings.Join(docNums, ",")
	params := url.Values{}
	params.Set("$select", "DocEntry,DocNum")
	params.Set("$filter", fmt.Sprintf("DocNum in (%s) and DocumentStatus eq 'bost_Open'", filterNums))
	queryURL := fmt.Sprintf("%s/%s?%s", s.BaseURL, endpoint, params.Encode())

	req, err := http.NewRequest(http.MethodGet, queryURL, nil)
	if err != nil {
		return nil, NewSAPConnectionError("consulta", err)
	}
	// Service Layer es sensible a estos headers; sin ellos puede devolver 406/415 en algunas versiones.
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, NewSAPConnectionError("consulta", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, NewSAPErrorFromResponseWithURL("consulta", resp.StatusCode, respBody, queryURL)
	}

	var mapping DocumentMapping
	if err := json.NewDecoder(resp.Body).Decode(&mapping); err != nil {
		return nil, NewSAPErrorFromResponseWithURL("consulta", resp.StatusCode, []byte("respuesta no es JSON válido: "+err.Error()), queryURL)
	}

	var docEntries []int
	for _, doc := range mapping.Value {
		docEntries = append(docEntries, doc.DocEntry)
	}

	return docEntries, nil
}

// CloseDocumentsBatch envía el $batch para cerrar los documentos resueltos
func (s *SAPClient) GetOpenTransferRequests() ([]TransferRequest, error) {
	params := url.Values{}
	params.Set("$select", "DocEntry,DocDate,DocNum,DocumentStatus")
	params.Set("$filter", "DocumentStatus eq 'bost_Open'")
	params.Set("$orderby", "DocNum desc")
	params.Set("$top", "100")
	queryURL := fmt.Sprintf("%s/InventoryTransferRequests?%s", s.BaseURL, params.Encode())

	req, err := http.NewRequest(http.MethodGet, queryURL, nil)
	if err != nil {
		return nil, NewSAPConnectionError("consulta", err)
	}
	// Service Layer puede devolver 406/415 si no se envía Accept explícito.
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, NewSAPConnectionError("consulta", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, NewSAPErrorFromResponseWithURL("consulta", resp.StatusCode, respBody, queryURL)
	}

	var list TransferRequestList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, NewSAPErrorFromResponseWithURL("consulta", resp.StatusCode, []byte("respuesta no es JSON válido: "+err.Error()), queryURL)
	}

	return list.Value, nil
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