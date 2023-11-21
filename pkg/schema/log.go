package schema

import "github.com/icinga/icinga-kubernetes/pkg/contracts"

type Log struct {
	kmetaWithoutNamespace
	Id            []byte
	ReferenceId   []byte
	ContainerName string
	Time          string
	Log           string
}

type Option func(*Log)

func NewLog(options ...Option) contracts.Resource {
	log := &Log{}

	for _, option := range options {
		option(log)
	}

	return log
}

func WithValues(id []byte, referenceId []byte, containerName string, time string, log string) func(*Log) {
	return func(l *Log) {
		l.Id = id
		l.ReferenceId = referenceId
		l.ContainerName = containerName
		l.Time = time
		l.Log = log
	}
}
