package output

type Output struct {
	Name string
	To   string
}

func NewOutput(name, to string) (*Output, error) {
	return &Output{
		Name: name,
		To:   to,
	}, nil
}
