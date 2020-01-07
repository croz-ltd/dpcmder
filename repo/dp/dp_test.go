package dp

import (
	"bytes"
	"encoding/json"
	"github.com/clbanning/mxj"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/utils/assert"
	"github.com/croz-ltd/dpcmder/utils/errs"
	"reflect"
	"testing"
)

func TestRemoveJSONKey(t *testing.T) {
	inputJSON := `{
  "keyok": "valok",
  "keyrem": {
     "bla": 11
  },
  "keysome1": {
    "asdf": 111,
    "keyrem": "valrem"
  },
  "keysome2": { "keyrem": "valrem" }
}`
	wantJSON := `{
  "keyok": "valok",
  "keysome1": { "asdf": 111 },
  "keysome2": {}
}`
	var prettyJSON bytes.Buffer
	json.Indent(&prettyJSON, []byte(wantJSON), "", "  ")
	wantJSON = prettyJSON.String()

	gotJSON := removeJSONKey(inputJSON, "keyrem")
	prettyJSON.Truncate(0)
	json.Indent(&prettyJSON, []byte(gotJSON), "", "  ")
	gotJSON = prettyJSON.String()

	if string(gotJSON) != wantJSON {
		t.Errorf("cleanJSONObject('%s'): got '%s', want '%s'", inputJSON, gotJSON, wantJSON)
	}
}

func TestCleanJSONObject(t *testing.T) {
	inputJSON := `{
  "_links": {
    "self": {
      "href": "/mgmt/config/tmp/XMLFirewallService/get_internal_js_xmlfw"
    },
    "doc": {
      "href": "/mgmt/docs/config/XMLFirewallService"
    }
  },
  "XMLFirewallService": {
    "name": "get_internal_js_xmlfw",
    "mAdminState": "enabled",
    "HTTPVersion": {
      "Front": "HTTP/1.1",
      "Back": "HTTP/1.1"
    },
    "DoChunkedUpload": "off",
    "DefaultParamNamespace": "http://www.datapower.com/param/config",
    "QueryParamNamespace": "http://www.datapower.com/param/query",
    "Type": "loopback-proxy",
    "XMLManager": {
      "value": "default",
      "href": "/mgmt/config/tmp/XMLManager/default"
    },
    "StylePolicy": {
      "value": "get_internal_js_xmlpolicy",
      "href": "/mgmt/config/tmp/StylePolicy/get_internal_js_xmlpolicy"
    },
    "MaxMessageSize": 0
	}
}`
	wantJSON := `{
  "XMLFirewallService": {
    "name": "get_internal_js_xmlfw",
    "mAdminState": "enabled",
    "HTTPVersion": {
      "Front": "HTTP/1.1",
      "Back": "HTTP/1.1"
    },
    "DoChunkedUpload": "off",
    "DefaultParamNamespace": "http://www.datapower.com/param/config",
    "QueryParamNamespace": "http://www.datapower.com/param/query",
    "Type": "loopback-proxy",
    "XMLManager": {
      "value": "default"
    },
    "StylePolicy": {
      "value": "get_internal_js_xmlpolicy"
    },
    "MaxMessageSize": 0
	}
}`
	var prettyJSON bytes.Buffer
	json.Indent(&prettyJSON, []byte(wantJSON), "", "  ")
	wantJSON = prettyJSON.String()

	gotJSON, err := cleanJSONObject(inputJSON)
	prettyJSON.Truncate(0)
	json.Indent(&prettyJSON, []byte(gotJSON), "", "  ")
	gotJSON = prettyJSON.Bytes()
	if err != nil {
		t.Errorf("cleanJSONObject('%s'): got error %v", inputJSON, err)
		return
	}
	if string(gotJSON) != wantJSON {
		t.Errorf("cleanJSONObject('%s'): got '%s', want '%s'", inputJSON, gotJSON, wantJSON)
	}
}

func TestCleanXML(t *testing.T) {
	inputXML := `<XMLFirewallService xmlns:_xmlns="xmlns" _xmlns:env="http://www.w3.org/2003/05/soap-envelope" name="parse-cert">
  <mAdminState>enabled</mAdminState>
  <LocalAddress>0.0.0.0</LocalAddress>
  <HTTPVersion>
    <Front>HTTP/1.1</Front>
    <Back>HTTP/1.1</Back>
  </HTTPVersion>
  <DefaultParamNamespace>http://www.datapower.com/param/config</DefaultParamNamespace>
  <DebugMode persisted="false">off</DebugMode>
  <XMLManager class="XMLManager">default</XMLManager>
</XMLFirewallService>`
	wantXML := `<XMLFirewallService name="parse-cert">
<mAdminState>enabled</mAdminState>
<LocalAddress>0.0.0.0</LocalAddress>
<HTTPVersion>
  <Front>HTTP/1.1</Front>
  <Back>HTTP/1.1</Back>
</HTTPVersion>
<DefaultParamNamespace>http://www.datapower.com/param/config</DefaultParamNamespace>
<DebugMode>off</DebugMode>
<XMLManager class="XMLManager">default</XMLManager>
</XMLFirewallService>`

	gotXML, _ := cleanXML(inputXML)

	gotXMLBytes, _ := mxj.BeautifyXml([]byte(gotXML), "", "  ")
	wantXMLBytes, _ := mxj.BeautifyXml([]byte(wantXML), "", "  ")
	gotXML = string(gotXMLBytes)
	wantXML = string(wantXMLBytes)

	if gotXML != wantXML {
		t.Errorf("for cleanXML('%s'): got '%s', want '%s'", inputXML, gotXML, wantXML)
	}
}

func TestCleanXMLMgmtInterface(t *testing.T) {
	inputXML := `<WebGUI name="WebGUI-Settings" intrinsic="true">
<mAdminState>enabled</mAdminState>
<LocalAddress>0.0.0.0</LocalAddress>
<LocalPort>9090</LocalPort>
<SaveConfigOverwrites>on</SaveConfigOverwrites>
<IdleTimeout>60000</IdleTimeout>
<ACL class="AccessControlList">web-mgmt</ACL>
<SSLServerConfigType>server</SSLServerConfigType>
<EnableSTS>on</EnableSTS>
<XMLFirewall class="XMLFirewallService">web-mgmt</XMLFirewall>
</WebGUI>`
	wantXML := `<WebGUI name="WebGUI-Settings" intrinsic="true">
<mAdminState>enabled</mAdminState>
<LocalAddress>0.0.0.0</LocalAddress>
<LocalPort>9090</LocalPort>
<SaveConfigOverwrites>on</SaveConfigOverwrites>
<IdleTimeout>60000</IdleTimeout>
<ACL class="AccessControlList">web-mgmt</ACL>
<SSLServerConfigType>server</SSLServerConfigType>
<EnableSTS>on</EnableSTS>
</WebGUI>`

	gotXML, _ := cleanXML(inputXML)

	gotXMLBytes, _ := mxj.BeautifyXml([]byte(gotXML), "", "  ")
	wantXMLBytes, _ := mxj.BeautifyXml([]byte(wantXML), "", "  ")
	gotXML = string(gotXMLBytes)
	wantXML = string(wantXMLBytes)

	if gotXML != wantXML {
		t.Errorf("for cleanXML('%s'): got '%s', want '%s'", inputXML, gotXML, wantXML)
	}
}

func TestSplitOnFirst(t *testing.T) {
	testDataMatrix := [][]string{
		{"/usr/bin/share", "/", "", "usr/bin/share"},
		{"usr/bin/share", "/", "usr", "bin/share"},
		{"/share", "/", "", "share"},
		{"share", "/", "share", ""},
		{"my big testing task", " ", "my", "big testing task"},
	}
	for _, testCase := range testDataMatrix {
		gotPreffix, gotSuffix := splitOnFirst(testCase[0], testCase[1])
		if gotPreffix != testCase[2] || gotSuffix != testCase[3] {
			t.Errorf("for SplitOnFirst('%s', '%s'): got ('%s', '%s'), want ('%s', '%s')", testCase[0], testCase[1], gotPreffix, gotSuffix, testCase[2], testCase[3])
		}
	}
}

func TestSplitOnLast(t *testing.T) {
	testDataMatrix := [][]string{
		{"/usr/bin/share", "/", "/usr/bin", "share"},
		{"usr/bin/share", "/", "usr/bin", "share"},
		{"/share", "/", "", "share"},
		{"share", "/", "share", ""},
		{"my big testing task", " ", "my big testing", "task"},
		{"local:/test1/test2", "/", "local:/test1", "test2"},
		{"local:/test1", "/", "local:", "test1"},
		{"local:", "/", "local:", ""},
	}
	for _, testCase := range testDataMatrix {
		gotPreffix, gotSuffix := splitOnLast(testCase[0], testCase[1])
		if gotPreffix != testCase[2] || gotSuffix != testCase[3] {
			t.Errorf("for SplitOnLast('%s', '%s'): got ('%s', '%s'), want ('%s', '%s')", testCase[0], testCase[1], gotPreffix, gotSuffix, testCase[2], testCase[3])
		}
	}
}

func TestGetViewConfigByPath(t *testing.T) {
	testDataMatrix := []struct {
		currentView *model.ItemConfig
		dirPath     string
		newView     *model.ItemConfig
		err         error
	}{
		{&model.ItemConfig{Type: model.ItemNone}, "", nil, errs.Error("Can't get view for path '' if DataPower domain is not selected.")},
		{&model.ItemConfig{Type: model.ItemDpConfiguration}, "", nil, errs.Error("Can't get view for path '' if DataPower domain is not selected.")},
		{&model.ItemConfig{Type: model.ItemDpDomain, DpDomain: "default"}, "", &model.ItemConfig{Type: model.ItemDpDomain, DpDomain: "default"}, nil},
		{&model.ItemConfig{
			Type: model.ItemDpFilestore, DpDomain: "default", DpFilestore: "cert:",
			Parent: &model.ItemConfig{Type: model.ItemDpDomain, DpDomain: "default"}}, "",
			&model.ItemConfig{Type: model.ItemDpDomain, DpDomain: "default"}, nil},
		{&model.ItemConfig{
			Type: model.ItemDpFilestore, DpDomain: "default", DpFilestore: "local:",
			Path:   "/dir1/dir2",
			Parent: &model.ItemConfig{Type: model.ItemDpDomain, DpDomain: "default"}}, "",
			&model.ItemConfig{Type: model.ItemDpDomain, DpDomain: "default"}, nil},
		{&model.ItemConfig{
			Type: model.ItemDpFilestore, DpDomain: "default", DpFilestore: "cert:",
			Parent: &model.ItemConfig{Type: model.ItemDpDomain, DpDomain: "default"}},
			"local:/dirA/dirB",
			&model.ItemConfig{
				Type: model.ItemDirectory, DpDomain: "default", DpFilestore: "local:",
				Path: "local:/dirA/dirB",
				Parent: &model.ItemConfig{
					Type: model.ItemDirectory, DpDomain: "default", DpFilestore: "local:",
					Path: "local:/dirA",
					Parent: &model.ItemConfig{
						Type: model.ItemDpFilestore, DpDomain: "default", DpFilestore: "local:",
						Path: "local:", Parent: &model.ItemConfig{
							Type: model.ItemDpDomain, DpDomain: "default"}}}},
			nil},
	}

	for _, testCase := range testDataMatrix {
		newView, err := Repo.GetViewConfigByPath(testCase.currentView, testCase.dirPath)
		if !reflect.DeepEqual(newView, testCase.newView) {
			t.Errorf("for GetViewConfigByPath(%v, '%s') res: got %v, want %v", testCase.currentView, testCase.dirPath, newView, testCase.newView)
		}
		if !reflect.DeepEqual(err, testCase.err) {
			t.Errorf("for GetViewConfigByPath(%v, '%s') err: got %v, want %v", testCase.currentView, testCase.dirPath, err, testCase.err)
		}
	}
}

func TestRenameObject(t *testing.T) {
	objectJSONInput := `{
	"XMLFirewallService": {
		"name": "example-Firewall5",
		"mAdminState": "enabled",
		"LocalAddress": "0.0.0.0",
		"UserSummary": "an example XML Firewall Service no 5",
		"Priority": "normal"}
}`
	objectJSONExpected := `{
	"XMLFirewallService": {
		"name": "new-xmlfw-name",
		"mAdminState": "enabled",
		"LocalAddress": "0.0.0.0",
		"UserSummary": "an example XML Firewall Service no 5",
		"Priority": "normal"}
}`
	Repo.dataPowerAppliance.RestUrl = ""
	Repo.dataPowerAppliance.SomaUrl = ""
	objectJSONGotErr, err := Repo.RenameObject([]byte(objectJSONInput), "new-xmlfw-name")
	assert.DeepEqual(t, "JSON object configuration rename err", err, errs.Error("DataPower management interface not set."))
	assert.DeepEqual(t, "JSON object configuration rename", objectJSONGotErr, []byte(nil))

	Repo.dataPowerAppliance.RestUrl = "https://some.rest.url"
	objectJSONGot, err := Repo.RenameObject([]byte(objectJSONInput), "new-xmlfw-name")
	assert.DeepEqual(t, "JSON object configuration rename err", err, nil)
	assert.DeepEqual(t, "JSON object configuration rename", string(objectJSONGot), objectJSONExpected)

	objectXMLInput := `<XMLFirewallService name="example-Firewall5">
  <mAdminState>enabled</mAdminState>
  <LocalAddress>0.0.0.0</LocalAddress>
  <UserSummary>an example XML Firewall Service no 5</UserSummary>
  <Priority>normal</Priority>
</XMLFirewallService>`
	objectXMLExpected := `<XMLFirewallService name="new-xmlfw-name">
  <mAdminState>enabled</mAdminState>
  <LocalAddress>0.0.0.0</LocalAddress>
  <UserSummary>an example XML Firewall Service no 5</UserSummary>
  <Priority>normal</Priority>
</XMLFirewallService>`
	Repo.dataPowerAppliance.RestUrl = ""
	Repo.dataPowerAppliance.SomaUrl = ""
	objectXMLGotErr, err := Repo.RenameObject([]byte(objectXMLInput), "new-xmlfw-name")
	assert.DeepEqual(t, "XML object configuration rename err", err, errs.Error("DataPower management interface not set."))
	assert.DeepEqual(t, "XML object configuration rename", objectXMLGotErr, []byte(nil))

	Repo.dataPowerAppliance.SomaUrl = "https://some.soma.url"
	objectXMLGot, err := Repo.RenameObject([]byte(objectXMLInput), "new-xmlfw-name")
	assert.DeepEqual(t, "XML object configuration rename err", err, nil)
	assert.DeepEqual(t, "XML object configuration rename", string(objectXMLGot), objectXMLExpected)
}
