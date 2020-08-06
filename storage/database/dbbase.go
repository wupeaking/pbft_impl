package database

type DB interface {
	Get(key string) (string, error)
	Set(key, value string) error
	Delete(key string) error
	// Open(filename string) error
}
