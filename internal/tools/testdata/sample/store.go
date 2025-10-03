package sample

type Storage interface {
	Save(key string, value string) error
	Load(key string) (string, error)
}
