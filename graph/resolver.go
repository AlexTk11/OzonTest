package graph

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

import "PostAndComment/storage"

type Resolver struct {
	Storage storage.Storage
}
