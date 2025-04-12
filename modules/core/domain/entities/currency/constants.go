package currency

var (
	USD = Currency{
		Code:   UsdCode,
		Name:   "US Dollar",
		Symbol: UsdSymbol,
	}
	EUR = Currency{
		Code:   EurCode,
		Name:   "Euro",
		Symbol: EurSymbol,
	}
	TRY = Currency{
		Code:   TryCode,
		Name:   "Turkish Lira",
		Symbol: TrySymbol,
	}
	GBP = Currency{
		Code:   GbpCode,
		Name:   "British Pound",
		Symbol: GbpSymbol,
	}
	RUB = Currency{
		Code:   RubCode,
		Name:   "Russian Ruble",
		Symbol: RubSymbol,
	}
	JPY = Currency{
		Code:   JpyCode,
		Name:   "Japanese Yen",
		Symbol: JpySymbol,
	}
	CNY = Currency{
		Code:   CnyCode,
		Name:   "Chinese Yuan",
		Symbol: CnySymbol,
	}
	UZS = Currency{
		Code:   UzsCode,
		Name:   "Som",
		Symbol: UzsSymbol,
	}
	AUD = Currency{
		Code:   AudCode,
		Name:   "Australian Dollar",
		Symbol: AudSymbol,
	}
	CAD = Currency{
		Code:   CadCode,
		Name:   "Canadian Dollar",
		Symbol: CadSymbol,
	}
	CHF = Currency{
		Code:   ChfCode,
		Name:   "Swiss Franc",
		Symbol: ChfSymbol,
	}
)

var (
	ValidCodes = []Code{
		UsdCode, EurCode, RubCode, TryCode, UzsCode, GbpCode, AudCode, CadCode, ChfCode, CnyCode, JpyCode,
	}
	ValidSymbols = []Symbol{
		UsdSymbol, EurSymbol, RubSymbol, TrySymbol, UzsSymbol, GbpSymbol, AudSymbol, CadSymbol, ChfSymbol, CnySymbol,
		JpySymbol,
	}
	Currencies = []Currency{
		USD,
		EUR,
		TRY,
		GBP,
		RUB,
		JPY,
		CNY,
		CHF,
		AUD,
		CAD,
		CHF,
		UZS,
	}
)
