package entity

import "encoding/json"

// PricingTierEntry — satu level tarif bertingkat
type PricingTierEntry struct {
	StartMinute int     `json:"startMinute"` // menit mulai tier (inklusif)
	EndMinute   *int    `json:"endMinute"`   // menit akhir tier (eksklusif), null = unlimited
	Price       float64 `json:"price"`       // harga per jam untuk tier ini
}

// PricingTierList adalah array dari PricingTierEntry untuk scan/save JSONB
type PricingTierList []PricingTierEntry

func (p *PricingTierList) Scan(value interface{}) error {
	if value == nil {
		*p = PricingTierList{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, p)
}

func (p PricingTierList) Value() (interface{}, error) {
	if len(p) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(p)
}
