package passport

func New(series, number string) Passport {
	return &passport{
		series: series,
		number: number,
	}
}

type passport struct {
	series string
	number string
}

func (p *passport) Series() string {
	return p.series
}

func (p *passport) Number() string {
	return p.number
}

func (p *passport) Identifier() string {
	return p.series + p.number
}
