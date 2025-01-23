package controller

type Controller struct {
	/*basePath string*/
}

/*func (c *Controller) SetBasePath(basePath string) *Controller {
	c.basePath = basePath
	return c
}

func (c Controller) BasePath() string {
	return c.basePath
}

func (c Controller) FullPath(controller any) string {
	basePath := c.BasePath()
	if basePath == "" {
		return c.Endpoint(controller)
	}
	return basePath + "/" + c.Endpoint(controller)
}*/
