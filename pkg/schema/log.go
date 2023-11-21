package schema

type Log struct {
	kmetaWithoutNamespace
	Id            []byte
	ReferenceId   []byte
	ContainerName string
	Time          string
	Log           string
}
