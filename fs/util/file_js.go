package util

import (
	"bytes"
	"io/ioutil"
	"net/http"
)

type file struct {
	*bytes.Reader
}

func (f *file) Close() error {
	return nil
}

func OpenFile(path string) (ReadSeekCloser, error) {
	res, err := http.Get(path)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	f := &file{bytes.NewReader(body)}
	return f, nil
}
