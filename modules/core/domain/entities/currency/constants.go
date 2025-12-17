package currency

var (
	USD = New(UsdCode, "US Dollar", UsdSymbol)
	EUR = New(EurCode, "Euro", EurSymbol)
	TRY = New(TryCode, "Turkish Lira", TrySymbol)
	GBP = New(GbpCode, "British Pound", GbpSymbol)
	RUB = New(RubCode, "Russian Ruble", RubSymbol)
	JPY = New(JpyCode, "Japanese Yen", JpySymbol)
	CNY = New(CnyCode, "Chinese Yuan", CnySymbol)
	UZS = New(UzsCode, "Som", UzsSymbol)
	AUD = New(AudCode, "Australian Dollar", AudSymbol)
	CAD = New(CadCode, "Canadian Dollar", CadSymbol)
	CHF = New(ChfCode, "Swiss Franc", ChfSymbol)
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
