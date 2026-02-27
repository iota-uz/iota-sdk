package position

// Upload is an interface that represents an uploaded file in the warehouse module.
// It is used to decouple the warehouse module from the core module's upload entity.
type Upload interface {
	// ID returns the unique identifier of the upload.
	ID() uint
	// URL returns the public URL of the upload.
	URL() string
	// Mimetype returns the MIME type of the upload.
	Mimetype() string
	// Size returns the human-readable size of the upload.
	Size() string
	// Hash returns the unique hash of the upload content.
	Hash() string
	// Slug returns the URL-friendly identifier of the upload.
	Slug() string
}
