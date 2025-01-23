package interfaces

import "net/http"

type GetHandler interface {
	Get(w http.ResponseWriter, r *http.Request)
}

type GetOneHandler interface {
	GetOne(w http.ResponseWriter, r *http.Request)
}

type GetStructureHandler interface {
	GetStructure(w http.ResponseWriter, r *http.Request)
}

type GetRelStructureHandler interface {
	GetRelStructure(w http.ResponseWriter, r *http.Request)
}

type PostHandler interface {
	Post(w http.ResponseWriter, r *http.Request)
}

type PatchHandler interface {
	Patch(w http.ResponseWriter, r *http.Request)
}

type PatchOneHandler interface {
	PatchOne(w http.ResponseWriter, r *http.Request)
}

type DeleteHandler interface {
	Delete(w http.ResponseWriter, r *http.Request)
}
