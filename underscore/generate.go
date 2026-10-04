package underscore

// Pin the Underscore version so `go generate` is reproducible; an unpinned
// "latest" URL drifts whenever upstream publishes a new release.
//go:generate go run download.go --url https://cdn.jsdelivr.net/npm/underscore@1.13.7/underscore-min.js --output underscore-min.js
//go:generate go run download.go --url https://raw.githubusercontent.com/jashkenas/underscore/1.13.7/LICENSE --output LICENSE.underscorejs
