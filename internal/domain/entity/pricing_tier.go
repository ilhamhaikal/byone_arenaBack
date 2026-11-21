package entity

import (
	"encoding/json"
	"fmt"
)

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

// Validate memastikan tarif bertingkat valid: dimulai dari menit 0, berurutan,
// tidak overlap, tidak ada gap, dan harga tidak negatif.
func (p PricingTierList) Validate() error {
	if len(p) == 0 {
		return nil
	}
	if p[0].StartMinute != 0 {
		return fmt.Errorf("tarif bertingkat harus dimulai dari menit 0")
	}

	for i, tier := range p {
		if tier.StartMinute < 0 {
			return fmt.Errorf("tier %d: startMinute tidak boleh negatif", i+1)
		}
		if tier.Price < 0 {
			return fmt.Errorf("tier %d: harga tidak boleh negatif", i+1)
		}
		if tier.EndMinute == nil {
			if i != len(p)-1 {
				return fmt.Errorf("tier %d: endMinute kosong hanya boleh di tier terakhir", i+1)
			}
			continue
		}
		if *tier.EndMinute <= tier.StartMinute {
			return fmt.Errorf("tier %d: endMinute harus lebih besar dari startMinute", i+1)
		}
		if i < len(p)-1 {
			if *tier.EndMinute > p[i+1].StartMinute {
				return fmt.Errorf("tier %d dan %d overlap", i+1, i+2)
			}
			if *tier.EndMinute < p[i+1].StartMinute {
				return fmt.Errorf("tier %d dan %d memiliki celah menit", i+1, i+2)
			}
		}
	}
	return nil
}
