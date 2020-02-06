package dp

import (
	"fmt"
	"io/ioutil"
)

type mockRequester struct{}

func (nr mockRequester) httpRequest(dpa dpApplicance, urlFullPath, method, body string) (string, error) {
	fmt.Println(urlFullPath)
	var content []byte
	var err error

	switch urlFullPath {
	case "https://my_dp_host:5554/mgmt/status/MyDomain/ObjectStatus":
		content, err = ioutil.ReadFile("testdata/object_class_list.json")
	case "https://my_dp_host:5550/service/mgmt/current":
		content, err = ioutil.ReadFile("testdata/object_class_list.xml")
	default:
		fmt.Printf("Unrecognized urlFullPath '%s'.\n", urlFullPath)
	}

	return string(content), err
}
