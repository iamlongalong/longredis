package longredis

type Client struct {
	*client
}

func (c *Client) SubPrefix(prefix string) *Client {
	return &Client{client: c.client.SubPrefix(prefix)}
}

func NewClient(opt Option) (*Client, error) {
	c, err := newClient(opt)
	return &Client{client: c}, err
}
