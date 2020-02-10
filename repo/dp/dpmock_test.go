package dp

import (
	"fmt"
	"github.com/croz-ltd/dpcmder/utils/errs"
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
	case "https://my_dp_host:5554/mgmt/config/default/Domain":
		content, err = ioutil.ReadFile("testdata/domain_config_list.json")
	case "https://my_dp_host:5554/mgmt/status/default/DomainStatus":
		content, err = ioutil.ReadFile("testdata/domain_status_list.json")
	case "https://my_dp_host:5554/mgmt/filestore/test":
		content, err = ioutil.ReadFile("testdata/filestore_list.json")
	case "https://my_dp_host:5554/mgmt/filestore/test/store":
		content, err = ioutil.ReadFile("testdata/filestore_store_list.json")
	case "https://my_dp_host:5554/mgmt/filestore/test/store/gatewayscript":
		content, err = ioutil.ReadFile("testdata/filestore_store_gatewayscript_list.json")
	case "https://my_dp_host:5554/mgmt/filestore/test/store/gatewayscript/example-context.js":
		content, err = ioutil.ReadFile("testdata/get_file_gatewayscript_example_context.js")
	case "https://my_dp_host:5554/mgmt/filestore/test/store/gatewayscript/non-existing-file.js":
		content, err = ioutil.ReadFile("testdata/non_existing_resource.json")
	case "https://my_dp_host:5550/service/mgmt/current":
		var opTag string
		var opClass string
		var opObjClass string
		var opLayoutOnly string
		var opFilePath string

		r := regexp.MustCompile(`.*<man:([^ ]+) class="([^ ]+)"( object-class="([^ ]+)")?/>.*`)
		matches := r.FindStringSubmatch(body)
		if len(matches) == 5 {
			opTag = matches[1]
			opClass = matches[2]
			opObjClass = matches[4]
		}

		if len(matches) == 0 {
			r = regexp.MustCompile(`.*<man:(get-filestore) layout-only="([^ ]+)".*`)
			matches = r.FindStringSubmatch(body)
			if len(matches) == 3 {
				opTag = matches[1]
				opLayoutOnly = matches[2]
			}
		}

		if len(matches) == 0 {
			r = regexp.MustCompile(`.*<man:(get-file) name="([^ ]+)".*`)
			matches = r.FindStringSubmatch(body)
			if len(matches) == 3 {
				opTag = matches[1]
				opFilePath = matches[2]
			}
		}

		if len(matches) == 0 {
			fmt.Printf("dpmock_test: Unrecognized body of SOMA request:\n'%s'\n", body)
			return "", errs.Error("dpmock_test: Unrecognized body of SOMA request")
		}
		// fmt.Printf(" opTag: '%s', opClass: '%s', opObjClass: '%s', opLayoutOnly: '%s', opFilePath: '%s'.\n",
		// 	opTag, opClass, opObjClass, opLayoutOnly, opFilePath)
		switch {
		case opTag == "get-status" && opClass == "ObjectStatus" && opObjClass == "":
			content, err = ioutil.ReadFile("testdata/object_class_status_list.xml")
		case opTag == "get-status" && opClass == "ObjectStatus" && opObjClass == "XMLFirewallService":
			content, err = ioutil.ReadFile("testdata/object_xmlfwsvc_status_list.xml")
		case opTag == "get-config" && opClass == "XMLFirewallService" && opObjClass == "":
			content, err = ioutil.ReadFile("testdata/object_xmlfwsvc_config_list.xml")
		case opTag == "get-config" && opClass == "Domain" && opObjClass == "":
			content, err = ioutil.ReadFile("testdata/domain_config_list.xml")
		case opTag == "get-status" && opClass == "DomainStatus" && opObjClass == "":
			content, err = ioutil.ReadFile("testdata/domain_status_list.xml")
		case opTag == "get-filestore" && opLayoutOnly == "true":
			content, err = ioutil.ReadFile("testdata/filestore_layout_list.xml")
		case opTag == "get-filestore" && opLayoutOnly == "false":
			content, err = ioutil.ReadFile("testdata/filestore_all_list.xml")
		case opTag == "get-file" && opFilePath == "store:/gatewayscript/example-context.js":
			content, err = ioutil.ReadFile("testdata/get_file_gatewayscript_example_context.xml")
		case opTag == "get-file" && opFilePath == "store:/gatewayscript/non-existing-file.js":
			content, err = ioutil.ReadFile("testdata/non_existing_resource.json")
		default:
			fmt.Printf("dpmock_test: Unrecognized SOMA request opTag: '%s', opClass: '%s', opObjClass: '%s'.\n",
				opTag, opClass, opObjClass)
		}

		// <man:get-status class="ObjectStatus" object-class="%s"/>
	default:
		fmt.Printf("dpmock_test: Unrecognized urlFullPath '%s'.\n", urlFullPath)
	}

	return string(content), err
}
