// Package e2e holds the end-to-end scenarios of the platform: whole runs, one
// process, no Docker (design.md §10, Makefile target test-e2e).
//
// Every test here is behind the e2e build tag, so an ordinary go test ./...
// does not start processes or bind ports. This file carries no code and exists
// so that the directory is a package even with the tag off: a package whose
// every file is excluded by a build constraint is an error of go build ./...,
// not an empty package.
package e2e
