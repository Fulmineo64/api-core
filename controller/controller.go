package controller

type Controller struct {
	path string
}

func (c *Controller) SetPath(path string) {
	c.path = path
}

func (c *Controller) Path() string {
	return c.path
}
