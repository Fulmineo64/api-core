package fetch

func Get(urlSegments ...string) *FetchRequest {
	fetchRequest := FetchRequest{}
	return fetchRequest.Get(urlSegments...)
}

func Patch(urlSegments ...string) *FetchRequest {
	fetchRequest := FetchRequest{}
	return fetchRequest.Patch(urlSegments...)
}

func Post(urlSegments ...string) *FetchRequest {
	fetchRequest := FetchRequest{}
	return fetchRequest.Post(urlSegments...)
}

func Delete(urlSegments ...string) *FetchRequest {
	fetchRequest := FetchRequest{}
	return fetchRequest.Delete(urlSegments...)
}
