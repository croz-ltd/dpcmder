package dp

import (
	"fmt"
	"io/ioutil"
	"regexp"
)

type mockRequester struct{}

func (nr mockRequester) httpRequest(dpa dpApplicance, urlFullPath, method, body string) (string, error) {
	// fmt.Println(urlFullPath)
	var content []byte
	var err error

	switch urlFullPath {
	case "https://my_dp_host:5554/mgmt/status/MyDomain/ObjectStatus":
		content, err = ioutil.ReadFile("testdata/object_class_status_list.json")
	case "https://my_dp_host:5554/mgmt/config/MyDomain/XMLFirewallService":
		content, err = ioutil.ReadFile("testdata/object_xmlfwsvc_config_list.json")
	case "https://my_dp_host:5550/service/mgmt/current":
		// var opTag string
		// var opClass string
		// var opObjClass string

		r := regexp.MustCompile(`.*<man:([^ ]+) class="([^ ]+)"( object-class="([^ ]+)")?/>.*`)
		matches := r.FindStringSubmatch(body)
		opTag := matches[1]
		opClass := matches[2]
		opObjClass := matches[4]
		// fmt.Printf(" opTag: '%s', opClass: '%s', opObjClass: '%s'.\n",
		// 	opTag, opClass, opObjClass)
		switch {
		case opTag == "get-status" && opClass == "ObjectStatus" && opObjClass == "":
			content, err = ioutil.ReadFile("testdata/object_class_status_list.xml")
		case opTag == "get-status" && opClass == "ObjectStatus" && opObjClass == "XMLFirewallService":
			content, err = ioutil.ReadFile("testdata/object_xmlfwsvc_status_list.xml")
		case opTag == "get-config" && opClass == "XMLFirewallService" && opObjClass == "":
			content, err = ioutil.ReadFile("testdata/object_xmlfwsvc_config_list.xml")
		default:
			fmt.Printf("unrecognized SOMA request opTag: '%s', opClass: '%s', opObjClass: '%s'.\n",
				opTag, opClass, opObjClass)
		}

		// <man:get-status class="ObjectStatus" object-class="%s"/>
	default:
		fmt.Printf("Unrecognized urlFullPath '%s'.\n", urlFullPath)
	}

	return string(content), err
}
