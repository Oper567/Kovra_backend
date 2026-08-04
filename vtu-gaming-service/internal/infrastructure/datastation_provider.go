package infrastructure

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lucepay-dev/lucepay/backend/vtu-gaming-service/internal/domain"
)

type DatastationProvider struct {
	client *DataStationClient
}

func NewDatastationProvider(apiKey string) *DatastationProvider {
	return &DatastationProvider{
		client: NewDataStationClient(apiKey),
	}
}

func (p *DatastationProvider) Execute(product *domain.Product, recipient string) (string, map[string]any, error) {
	switch product.Category {
	case domain.CategoryAirtime:
		network, _ := getIntFromMeta(product.Metadata, "network")
		if network == 0 {
			network = 1 // Default fallback
		}
		amount := int(product.Amount.IntPart())
		
		req := AirtimePinRequest{
			Network:       network,
			NetworkAmount: amount,
			Quantity:      1,
			NameOnCard:    "LucePay User",
		}
		resp, err := p.client.GenerateAirtimePin(req)
		return generateRef(), resp, err
		
	case domain.CategoryData:
		network, _ := getIntFromMeta(product.Metadata, "network")
		if network == 0 {
			network = 1
		}
		plan, _ := strconv.Atoi(product.ProviderCode)
		if plan == 0 {
			plan, _ = getIntFromMeta(product.Metadata, "plan")
		}
		
		req := BuyDataRequest{
			Network: network,
			Mobile:  recipient,
			Plan:    plan,
			Ported:  false,
		}
		resp, err := p.client.BuyData(req)
		return generateRef(), resp, err
		
	case domain.CategoryEducation:
		req := EduPinRequest{
			ExamName: strings.ToLower(product.Name),
			Quantity: 1,
		}
		resp, err := p.client.GenerateEduPin(req)
		return generateRef(), resp, err
		
	case domain.CategoryElectricity:
		// Not fully implemented in datastation client for buying, only validation
		// Just returning an error for now
		return "", nil, fmt.Errorf("electricity purchase not implemented for Datastation")
		
	default:
		return "", nil, fmt.Errorf("unsupported category %s for Datastation", product.Category)
	}
}

func (p *DatastationProvider) CheckStatus(providerRef string) (domain.OrderStatus, error) {
	// Assume instant fulfillment for now
	return domain.OrderCompleted, nil
}

func (p *DatastationProvider) Name() string {
	return "datastation"
}

func getIntFromMeta(meta map[string]any, key string) (int, bool) {
	if meta == nil {
		return 0, false
	}
	val, ok := meta[key]
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case string:
		i, err := strconv.Atoi(v)
		return i, err == nil
	default:
		return 0, false
	}
}

func generateRef() string {
	return fmt.Sprintf("DS-%d", time.Now().UnixNano())
}
