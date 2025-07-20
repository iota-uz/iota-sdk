package table

type SortDirection string

const (
	SortDirectionNone SortDirection = ""
	SortDirectionAsc  SortDirection = "asc"
	SortDirectionDesc SortDirection = "desc"
)

func (sd SortDirection) String() string {
	return string(sd)
}

func (sd SortDirection) IsAsc() bool {
	return sd == SortDirectionAsc
}

func (sd SortDirection) IsDesc() bool {
	return sd == SortDirectionDesc
}

func (sd SortDirection) IsNone() bool {
	return sd == SortDirectionNone
}

func ParseSortDirection(value string) SortDirection {
	switch value {
	case "asc":
		return SortDirectionAsc
	case "desc":
		return SortDirectionDesc
	default:
		return SortDirectionNone
	}
}
