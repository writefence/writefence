package store

type Doc struct {
	ID      string
	Summary string
}

type Store interface {
	ListDocs(limit int) ([]Doc, error)
	DeleteDocs(ids []string) error
}
