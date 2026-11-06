module github.com/ichnois/ichnois-go/negroni

go 1.26.0

replace github.com/ichnois/ichnois-go => ../

require (
	github.com/ichnois/ichnois-go v1.0.0-alpha.1
	github.com/google/go-cmp v0.7.0
	github.com/urfave/negroni/v3 v3.1.1
)

require (
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
)
