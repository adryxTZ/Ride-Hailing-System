package payment

// Gateways chooses the collection gateway for each currency (each country pays
// through its own: Selcom in Tanzania, another provider elsewhere) and finds the
// one a stored payment went through by name. A currency without one has no
// mobile-money payments.
type Gateways struct {
	byName     map[string]Gateway
	byCurrency map[string]Gateway
}

// NewGateways builds the registry from currency → gateway.
func NewGateways(byCurrency map[string]Gateway) *Gateways {
	g := &Gateways{byName: map[string]Gateway{}, byCurrency: map[string]Gateway{}}
	for cur, gw := range byCurrency {
		if gw == nil {
			continue
		}
		g.byCurrency[cur] = gw
		g.byName[gw.Name()] = gw
	}
	return g
}

// For returns the gateway that collects a currency.
func (g *Gateways) For(currency string) (Gateway, bool) {
	gw, ok := g.byCurrency[currency]
	return gw, ok
}

// Named returns a gateway by its key (a stored payment's gateway).
func (g *Gateways) Named(name string) (Gateway, bool) {
	gw, ok := g.byName[name]
	return gw, ok
}

// Payouters chooses the payout gateway for each currency and finds one by name.
type Payouters struct {
	byName     map[string]Payouter
	byCurrency map[string]Payouter
}

// NewPayouters builds the registry from currency → payout gateway.
func NewPayouters(byCurrency map[string]Payouter) *Payouters {
	p := &Payouters{byName: map[string]Payouter{}, byCurrency: map[string]Payouter{}}
	for cur, po := range byCurrency {
		if po == nil {
			continue
		}
		p.byCurrency[cur] = po
		p.byName[po.Name()] = po
	}
	return p
}

// For returns the payout gateway of a currency.
func (p *Payouters) For(currency string) (Payouter, bool) {
	po, ok := p.byCurrency[currency]
	return po, ok
}

// Named returns a payout gateway by its key.
func (p *Payouters) Named(name string) (Payouter, bool) {
	po, ok := p.byName[name]
	return po, ok
}

// Any reports whether any currency can be paid out.
func (p *Payouters) Any() bool { return p != nil && len(p.byCurrency) > 0 }
