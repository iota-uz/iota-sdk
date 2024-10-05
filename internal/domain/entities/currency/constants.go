package currency

var (
	USD = Currency{
		Code:   Code{UsdCode},
		Name:   "US Dollar",
		Symbol: Symbol{UsdSymbol},
	}
	EUR = Currency{
		Code:   Code{EurCode},
		Name:   "Euro",
		Symbol: Symbol{EurSymbol},
	}
	TRY = Currency{
		Code:   Code{TryCode},
		Name:   "Turkish Lira",
		Symbol: Symbol{TrySymbol},
	}
	GBP = Currency{
		Code:   Code{GbpCode},
		Name:   "British Pound",
		Symbol: Symbol{GbpSymbol},
	}
	RUB = Currency{
		Code:   Code{RubCode},
		Name:   "Russian Ruble",
		Symbol: Symbol{RubSymbol},
	}
	JPY = Currency{
		Code:   Code{JpyCode},
		Name:   "Japanese Yen",
		Symbol: Symbol{JpySymbol},
	}
	CNY = Currency{
		Code:   Code{CnyCode},
		Name:   "Chinese Yuan",
		Symbol: Symbol{CnySymbol},
	}
	SOM = Currency{
		Code:   Code{SomCode},
		Name:   "Som",
		Symbol: Symbol{SomSymbol},
	}
	AUD = Currency{
		Code:   Code{AudCode},
		Name:   "Australian Dollar",
		Symbol: Symbol{AudSymbol},
	}
	CAD = Currency{
		Code:   Code{CadCode},
		Name:   "Canadian Dollar",
		Symbol: Symbol{CadSymbol},
	}
	CHF = Currency{
		Code:   Code{ChfCode},
		Name:   "Swiss Franc",
		Symbol: Symbol{ChfSymbol},
	}
)

var (
	ValidCodes   = []CodeEnum{UsdCode, EurCode, RubCode, TryCode, SomCode, GbpCode, AudCode, CadCode, ChfCode, CnyCode, JpyCode}
	ValidSymbols = []SymbolEnum{UsdSymbol, EurSymbol, RubSymbol, TrySymbol, SomSymbol, GbpSymbol, AudSymbol, CadSymbol, ChfSymbol, CnySymbol, JpySymbol}
	Currencies   = []Currency{
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
		SOM,
	}
)
