package fatturapa

import (
	"strconv"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/regimes/it"
	"github.com/invopop/gobl/tax"
)

// datiBeniServizi contains all data related to the goods and services sold.
type datiBeniServizi struct {
	DettaglioLinee []*dettaglioLinee
	DatiRiepilogo  []*datiRiepilogo
}

// dettaglioLinee contains line data such as description, quantity, price, etc.
type dettaglioLinee struct {
	NumeroLinea         string                 `xml:"NumeroLinea"`
	Descrizione         string                 `xml:"Descrizione"`
	Quantita            string                 `xml:"Quantita"`
	PrezzoUnitario      string                 `xml:"PrezzoUnitario"`
	ScontoMaggiorazione []*scontoMaggiorazione `xml:"ScontoMaggiorazione,omitempty"`
	PrezzoTotale        string                 `xml:"PrezzoTotale"`
	AliquotaIVA         string                 `xml:"AliquotaIVA"`
	Natura              string                 `xml:"Natura,omitempty"`
}

// datiRiepilogo contains tax summary data such as tax rate, tax amount, etc.
type datiRiepilogo struct {
	AliquotaIVA          string
	Natura               string `xml:",omitempty"`
	ImponibileImporto    string
	Imposta              string
	RiferimentoNormativo string `xml:",omitempty"`
}

func newDatiBeniServizi(inv *bill.Invoice) *datiBeniServizi {
	return &datiBeniServizi{
		DettaglioLinee: generateLineDetails(inv),
		DatiRiepilogo:  generateTaxSummary(inv),
	}
}

func generateLineDetails(inv *bill.Invoice) []*dettaglioLinee {
	var dl []*dettaglioLinee

	for _, line := range inv.Lines {
		d := &dettaglioLinee{
			NumeroLinea:         strconv.Itoa(line.Index),
			Descrizione:         line.Item.Name,
			Quantita:            formatAmount(&line.Quantity),
			PrezzoUnitario:      formatAmount(&line.Item.Price),
			PrezzoTotale:        formatAmount(&line.Total),
			ScontoMaggiorazione: extractLinePriceAdjustments(line),
		}
		if line.Taxes != nil && len(line.Taxes) > 0 {
			vatTax := line.Taxes.Get(tax.CategoryVAT)
			if vatTax != nil {
				d.AliquotaIVA = formatPercentageWithZero(vatTax.Percent)
				d.Natura = vatTax.Ext[it.ExtKeySDINature].String()
			}
		}

		dl = append(dl, d)
	}

	return dl
}

func generateTaxSummary(inv *bill.Invoice) []*datiRiepilogo {
	var dr []*datiRiepilogo
	var vatRateTotals []*tax.RateTotal

	for _, cat := range inv.Totals.Taxes.Categories {
		if cat.Code == tax.CategoryVAT {
			vatRateTotals = cat.Rates
			break
		}
	}

	for _, rateTotal := range vatRateTotals {
		dr = append(dr, &datiRiepilogo{
			AliquotaIVA:          formatPercentageWithZero(rateTotal.Percent),
			Natura:               rateTotal.Ext[it.ExtKeySDINature].String(),
			ImponibileImporto:    formatAmount(&rateTotal.Base),
			Imposta:              formatAmount(&rateTotal.Amount),
			RiferimentoNormativo: findRiferimentoNormativo(rateTotal),
		})
	}

	return dr
}

func extractLinePriceAdjustments(line *bill.Line) []*scontoMaggiorazione {
	list := make([]*scontoMaggiorazione, 0)

	for _, discount := range line.Discounts {
		// Unlike most formats, FatturaPA applies the discount to the unit price
		// instead of the line sum.
		// Quick ref: https://fex-app.com/FatturaElettronica/FatturaElettronicaBody/DatiBeniServizi/DettaglioLinee/PrezzoTotale
		a := discount.Amount.Divide(line.Quantity)
		list = append(list, &scontoMaggiorazione{
			Tipo:        scontoMaggiorazioneTypeDiscount,
			Percentuale: formatPercentage(discount.Percent),
			Importo:     formatAmount(&a),
		})
	}

	for _, charge := range line.Charges {
		a := charge.Amount.Divide(line.Quantity)
		list = append(list, &scontoMaggiorazione{
			Tipo:        scontoMaggiorazioneTypeCharge,
			Percentuale: formatPercentage(charge.Percent),
			Importo:     formatAmount(&a),
		})
	}

	return list
}

func findRiferimentoNormativo(rateTotal *tax.RateTotal) string {
	def := regime.ExtensionDef(it.ExtKeySDINature)

	nature := rateTotal.Ext[it.ExtKeySDINature].Code()
	for _, c := range def.Codes {
		if c.Code == nature {
			return c.Name[i18n.IT]
		}
	}

	return ""
}
