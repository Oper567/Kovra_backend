package infrastructure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// DataStationClient acts as the wrapper for the DataStation API.
type DataStationClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewDataStationClient initializes a new client using the provided token.
func NewDataStationClient(apiKey string) *DataStationClient {
	return &DataStationClient{
		BaseURL:    "https://datastation.com.ng",
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// =======================================================================
// 1. VALIDATE METER NUMBER (GET REQUEST)
// =======================================================================

// ValidateMeter checks the validity of a meter before electricity payment.
func (c *DataStationClient) ValidateMeter(meterNumber, discoName, mType string) (map[string]interface{}, error) {
	// Construct the URL with query parameters
	endpoint := fmt.Sprintf("%s/ajax/validate_meter_number", c.BaseURL)
	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	q := reqURL.Query()
	q.Add("meternumber", meterNumber)
	q.Add("disconame", discoName)
	q.Add("mtype", mType) // Usually "prepaid" or "postpaid"
	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", reqURL.String(), nil)
	if err != nil {
		return nil, err
	}

	// Add Required Headers
	req.Header.Add("Authorization", "Token "+c.APIKey)
	req.Header.Add("Content-Type", "application/json")

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse JSON
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response JSON: %s", string(body))
	}

	return result, nil
}

// =======================================================================
// 2. GENERATE AIRTIME PIN (POST REQUEST)
// =======================================================================

// GenerateAirtimePin payload definition
type AirtimePinRequest struct {
	Network       int    `json:"network"`        // e.g., 1 for MTN
	NetworkAmount int    `json:"network_amount"` // The amount ID or integer
	Quantity      int    `json:"quantity"`
	NameOnCard    string `json:"name_on_card"`
}

func (c *DataStationClient) GenerateAirtimePin(reqData AirtimePinRequest) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("%s/api/rechargepin/", c.BaseURL)

	payloadBytes, err := json.Marshal(reqData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Token "+c.APIKey)
	req.Header.Add("Content-Type", "application/json")

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		// API sometimes returns 201 Created with text/HTML on poor implementations. We handle it safely.
		return map[string]interface{}{
			"status": "success",
			"raw":    string(body),
		}, nil
	}

	return result, nil
}

// =======================================================================
// 3. GENERATE RESULT/EDUCATION PINS (POST REQUEST)
// =======================================================================

type EduPinRequest struct {
	ExamName string `json:"exam_name"` // "waec", "neco", etc.
	Quantity int    `json:"quantity"`
}

func (c *DataStationClient) GenerateEduPin(reqData EduPinRequest) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("%s/api/epin/", c.BaseURL)

	payloadBytes, err := json.Marshal(reqData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Token "+c.APIKey)
	req.Header.Add("Content-Type", "application/json")

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return map[string]interface{}{
			"status": "success",
			"raw":    string(body),
		}, nil
	}

	return result, nil
}

// =======================================================================
// 4. BUY DATA PLAN (GENERIC POST REQUEST)
// =======================================================================

// BuyData payload definition (Inferring from DataStation's standard structure)
type BuyDataRequest struct {
	Network int    `json:"network"` // 1=MTN, 2=GLO, 3=9MOBILE, 4=AIRTEL
	Mobile  string `json:"mobile_number"`
	Plan    int    `json:"plan"`    // The provider_id from the database
	Ported  bool   `json:"Ported_number"`
}

// BuyData processes direct-to-phone data purchases.
func (c *DataStationClient) BuyData(reqData BuyDataRequest) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("%s/api/data/", c.BaseURL)

	payloadBytes, err := json.Marshal(reqData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Token "+c.APIKey)
	req.Header.Add("Content-Type", "application/json")

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response JSON: %s", string(body))
	}

	return result, nil
}