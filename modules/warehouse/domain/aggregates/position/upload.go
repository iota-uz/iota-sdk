package position

type Upload interface {
	ID() uint
	URL() string
	Mimetype() string
	Size() string
	Hash() string
	Slug() string
}
