package layout

import (
	"fmt"
	"math/big"
)

// AddressDomain is one hardware address domain supplied by the selected
// target: its canonical identity, the width of a domain address and the size
// of one addressable unit. Hardware declarations name a domain but never
// invent one, and domain addresses are not limited to the target `uint`.
//
// Rules:
//   - rules/platform/platform_model.md — § 10 (Amendment MD-111)
//   - rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§ 1(6)-1(7), 3(2), 4(1)
type AddressDomain struct {
	Identity    string
	AddressBits uint16
	UnitBits    uint16
	// Linear marks a numeric linear domain, the only kind the 0.1
	// hardware region schema can describe.
	Linear bool
}

// Validate rejects an incomplete domain description.
func (d AddressDomain) Validate() error {
	if d.Identity == "" {
		return fmt.Errorf("address domain requires an identity")
	}
	if d.AddressBits == 0 || d.AddressBits > 256 {
		return fmt.Errorf("address domain %s address width must be 1..256 bits, got %d", d.Identity, d.AddressBits)
	}
	if d.UnitBits == 0 {
		return fmt.Errorf("address domain %s requires a positive addressable unit", d.Identity)
	}
	return nil
}

// Limit is the number of addressable units of the domain, 2^AddressBits:
// every valid address is below it and every range [base, base+size) must end
// at or below it.
func (d AddressDomain) Limit() *big.Int {
	return new(big.Int).Lsh(big.NewInt(1), uint(d.AddressBits))
}

// AddressDomain returns the selected plan's domain with that identity.
func (p ResolvedScalarPlan) AddressDomain(identity string) (AddressDomain, bool) {
	for _, domain := range p.AddressDomains {
		if domain.Identity == identity {
			return domain, true
		}
	}
	return AddressDomain{}, false
}

func validateAddressDomains(domains []AddressDomain) error {
	seen := map[string]bool{}
	for _, domain := range domains {
		if err := domain.Validate(); err != nil {
			return err
		}
		if seen[domain.Identity] {
			return fmt.Errorf("duplicate address domain %s", domain.Identity)
		}
		seen[domain.Identity] = true
	}
	return nil
}
