package dp

import (
	"fmt"
	"io/ioutil"
)

type mockRequester struct{}

func (nr mockRequester) httpRequest(dpa dpApplicance, urlFullPath, method, body string) (string, error) {
	fmt.Println(urlFullPath)
	content, err := ioutil.ReadFile("testdata/object_class_list.json")
	return string(content), err
}
