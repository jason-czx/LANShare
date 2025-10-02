package v1

import "LANShare/model"

type WebDavService struct{}

func (g *WebDavService) NewWebDAVServiceApi(root string) *model.WebDAVService {
	return model.NewWebDAVService(root)
}
