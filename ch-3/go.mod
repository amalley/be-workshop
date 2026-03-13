module github.com/AMalley/be-workshop/ch-3

go 1.26.1

replace github.com/gocql/gocql => github.com/scylladb/gocql v1.17.3

require github.com/gocql/gocql v1.7.0

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.18.4 // indirect
	golang.org/x/crypto v0.49.0
	golang.org/x/sync v0.19.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
)
