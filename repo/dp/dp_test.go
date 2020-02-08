package dp

import (
	"bytes"
	"encoding/json"
	"github.com/clbanning/mxj"
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/utils/assert"
	"github.com/croz-ltd/dpcmder/utils/errs"
	"reflect"
	"testing"
)

const (
	testRestURL = "https://my_dp_host:5554"
	testSomaURL = "https://my_dp_host:5550"
)

func clearRepo() {
	Repo.dpFilestoreXmls = make(map[string]string)
	Repo.invalidateCache = false
	Repo.dataPowerAppliance = dpApplicance{}
	Repo.ObjectConfigMode = false
}

func TestString(t *testing.T) {
	clearRepo()

	assert.Equals(t, "Normal DataPower repo", Repo.String(), "DataPower")
	assert.Equals(t, "Sync DataPower repo", SyncRepo.String(), "SyncDataPower")
}

func TestGetInitialItem(t *testing.T) {
	t.Run("Showing list of configurations", func(t *testing.T) {
		clearRepo()

		ii, err := Repo.GetInitialItem()
		assert.Equals(t, "GetInitialItem", err, nil)
		assert.DeepEqual(t, "GetInitialItem",
			ii,
			model.Item{
				Name:   "List appliance configurations",
				Config: &model.ItemConfig{Type: model.ItemNone}})
	})

	t.Run("Showing DataPower domains", func(t *testing.T) {
		clearRepo()

		Repo.dataPowerAppliance.RestUrl = testRestURL
		Repo.dataPowerAppliance.name = "MyApplianceName"
		ii, err := Repo.GetInitialItem()
		assert.Equals(t, "GetInitialItem", err, nil)
		assert.DeepEqual(t, "GetInitialItem",
			ii,
			model.Item{
				Name: "MyApplianceName",
				Config: &model.ItemConfig{Type: model.ItemDpConfiguration,
					Name:        "MyApplianceName",
					DpAppliance: "MyApplianceName",
					Parent:      &model.ItemConfig{Type: model.ItemNone}}})
	})

	t.Run("Showing DataPower domain filestores", func(t *testing.T) {
		clearRepo()

		Repo.dataPowerAppliance.RestUrl = testRestURL
		Repo.dataPowerAppliance.Domain = "my_domain"
		Repo.dataPowerAppliance.name = "MyApplianceName"
		ii, err := Repo.GetInitialItem()
		assert.Equals(t, "GetInitialItem", err, nil)
		assert.DeepEqual(t, "GetInitialItem",
			ii,
			model.Item{
				Name: "my_domain",
				Config: &model.ItemConfig{Type: model.ItemDpDomain,
					Name:        "my_domain",
					DpAppliance: "MyApplianceName",
					DpDomain:    "my_domain",
					Parent:      &model.ItemConfig{Type: model.ItemNone}}})
	})
}

func TestGetTitle(t *testing.T) {
	t.Run("Initial title", func(t *testing.T) {
		clearRepo()

		itemToShow := model.ItemConfig{}
		title := Repo.GetTitle(&itemToShow)
		assert.Equals(t, "GetTitle", title, " @  -  () ")
	})

	t.Run("List domains title", func(t *testing.T) {
		clearRepo()

		Repo.dataPowerAppliance.RestUrl = testRestURL
		Repo.dataPowerAppliance.Username = "user"
		Repo.dataPowerAppliance.name = "xxx"
		itemToShow := model.ItemConfig{DpAppliance: "MyApplianceName"}
		title := Repo.GetTitle(&itemToShow)
		assert.Equals(t, "GetTitle", title,
			"user @ https://my_dp_host:5554 - MyApplianceName () ")
	})

	t.Run("List filestores title", func(t *testing.T) {
		clearRepo()

		Repo.dataPowerAppliance.RestUrl = testRestURL
		Repo.dataPowerAppliance.Username = "user"
		Repo.dataPowerAppliance.name = "xxx"
		Repo.dataPowerAppliance.Domain = "yyy"
		itemToShow := model.ItemConfig{DpAppliance: "MyApplianceName",
			DpDomain: "MyDomain"}
		title := Repo.GetTitle(&itemToShow)
		assert.Equals(t, "GetTitle", title,
			"user @ https://my_dp_host:5554 - MyApplianceName (MyDomain) ")
	})

	t.Run("List files title", func(t *testing.T) {
		clearRepo()

		Repo.dataPowerAppliance.SomaUrl = testSomaURL
		Repo.dataPowerAppliance.Username = "user"
		Repo.dataPowerAppliance.name = "xxx"
		Repo.dataPowerAppliance.Domain = "yyy"
		itemToShow := model.ItemConfig{DpAppliance: "MyApplianceName",
			DpDomain: "MyDomain", DpFilestore: "zzz", Path: "local:/config/etc"}
		title := Repo.GetTitle(&itemToShow)
		assert.Equals(t, "GetTitle", title,
			"user @ https://my_dp_host:5550 - MyApplianceName (MyDomain) local:/config/etc")
	})
}

func TestGetList(t *testing.T) {
	t.Run("ObjectConfigMode wrong item type", func(t *testing.T) {
		clearRepo()

		dpa := config.DataPowerAppliance{
			RestUrl:  testRestURL,
			Username: "user",
			Domain:   "xxx",
		}
		Repo.ObjectConfigMode = true
		Repo.req = mockRequester{}

		itemToShow := model.ItemConfig{Type: model.ItemDpObjectClassList}
		itemList, err := Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err,
			errs.Error("Internal error showing object config mode - missing dp appliance."))
		assert.Equals(t, "GetList", len(itemList), 0)

		itemToShow.DpAppliance = "MyApplianceName"
		config.Conf.DataPowerAppliances[itemToShow.DpAppliance] = dpa
		itemList, err = Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err,
			errs.Error("Internal error showing object config mode - missing domain."))
		assert.Equals(t, "GetList", len(itemList), 0)

		itemToShow.DpDomain = "MyDomain"
		itemToShow.Type = model.ItemDirectory
		itemList, err = Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err,
			errs.Error("Internal error showing object config mode - wrong view type."))
		assert.Equals(t, "GetList", len(itemList), 0)
	})

	t.Run("ObjectConfigMode/ObjectClassList REST", func(t *testing.T) {
		clearRepo()

		dpa := config.DataPowerAppliance{
			RestUrl:  testRestURL,
			Username: "user",
			Domain:   "xxx",
		}
		Repo.ObjectConfigMode = true
		Repo.req = mockRequester{}

		itemToShow := model.ItemConfig{Type: model.ItemDpObjectClassList,
			DpAppliance: "MyApplianceName",
			DpDomain:    "MyDomain",
			DpFilestore: "local:",
			Path:        "Object classes"}
		config.Conf.DataPowerAppliances[itemToShow.DpAppliance] = dpa
		itemList, err := Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err, nil)
		assert.Equals(t, "GetList", len(itemList), 43)

		if len(itemList) == 43 {
			parentItemConfig := model.ItemConfig{Type: model.ItemDpObjectClassList,
				Path: "Object classes", DpAppliance: "MyApplianceName",
				DpDomain: "MyDomain", DpFilestore: "local:"}
			assert.DeepEqual(t, "GetList", itemList[0],
				model.Item{Name: "AAAJWTValidator", Size: "1", Modified: "",
					Selected: false,
					Config: &model.ItemConfig{Type: model.ItemDpObjectClass,
						Name: "AAAJWTValidator", Path: "AAAJWTValidator",
						DpAppliance: "MyApplianceName", DpDomain: "MyDomain",
						Parent: &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[23],
				model.Item{Name: "LogLabel", Size: "102", Modified: "",
					Selected: false,
					Config: &model.ItemConfig{Type: model.ItemDpObjectClass,
						Name: "LogLabel", Path: "LogLabel",
						DpAppliance: "MyApplianceName", DpDomain: "MyDomain",
						Parent: &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[41],
				model.Item{Name: "XMLFirewallService", Size: "8", Modified: "*",
					Selected: false,
					Config: &model.ItemConfig{Type: model.ItemDpObjectClass,
						Name: "XMLFirewallService", Path: "XMLFirewallService",
						DpAppliance: "MyApplianceName", DpDomain: "MyDomain",
						Parent: &parentItemConfig}})
		}

		itemToShow.Type = model.ItemDirectory
		itemList, err = Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err,
			errs.Error("Internal error showing object config mode - wrong view type."))
		assert.Equals(t, "GetList", len(itemList), 0)
	})

	t.Run("ObjectConfigMode/ObjectClassList SOMA", func(t *testing.T) {
		clearRepo()

		dpa := config.DataPowerAppliance{
			SomaUrl:  testSomaURL,
			Username: "user",
			Domain:   "xxx",
		}
		Repo.ObjectConfigMode = true
		Repo.req = mockRequester{}

		itemToShow := model.ItemConfig{Type: model.ItemDpObjectClassList,
			DpAppliance: "MyApplianceName",
			DpDomain:    "MyDomain", DpFilestore: "local:", Path: "Object classes"}
		config.Conf.DataPowerAppliances[itemToShow.DpAppliance] = dpa
		itemList, err := Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err, nil)
		assert.Equals(t, "GetList", len(itemList), 43)

		if len(itemList) == 43 {
			parentItemConfig := model.ItemConfig{Type: model.ItemDpObjectClassList,
				Path: "Object classes", DpAppliance: "MyApplianceName",
				DpDomain: "MyDomain", DpFilestore: "local:"}
			assert.DeepEqual(t, "GetList", itemList[0],
				model.Item{Name: "AAAJWTValidator", Size: "1", Modified: "",
					Selected: false,
					Config: &model.ItemConfig{Type: model.ItemDpObjectClass,
						Name: "AAAJWTValidator", Path: "AAAJWTValidator",
						DpAppliance: "MyApplianceName", DpDomain: "MyDomain",
						Parent: &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[23],
				model.Item{Name: "LogLabel", Size: "102", Modified: "",
					Selected: false,
					Config: &model.ItemConfig{Type: model.ItemDpObjectClass,
						Name: "LogLabel", Path: "LogLabel",
						DpAppliance: "MyApplianceName", DpDomain: "MyDomain",
						Parent: &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[41],
				model.Item{Name: "XMLFirewallService", Size: "8", Modified: "*",
					Selected: false,
					Config: &model.ItemConfig{Type: model.ItemDpObjectClass,
						Name: "XMLFirewallService", Path: "XMLFirewallService",
						DpAppliance: "MyApplianceName", DpDomain: "MyDomain",
						Parent: &parentItemConfig}})
		}
	})

	t.Run("ObjectConfigMode/ObjectList REST", func(t *testing.T) {
		clearRepo()

		dpa := config.DataPowerAppliance{
			RestUrl:  testRestURL,
			Username: "user",
			Domain:   "xxx",
		}
		Repo.ObjectConfigMode = true
		Repo.req = mockRequester{}

		itemToShow := model.ItemConfig{Type: model.ItemDpObjectClass,
			DpAppliance: "MyApplianceName",
			DpDomain:    "MyDomain", DpFilestore: "local:", Path: "XMLFirewallService"}
		config.Conf.DataPowerAppliances[itemToShow.DpAppliance] = dpa
		itemList, err := Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err, nil)
		assert.Equals(t, "GetList", len(itemList), 9)

		if len(itemList) == 9 {
			parentItemConfig := model.ItemConfig{Type: model.ItemDpObjectClass,
				Path: "XMLFirewallService", DpAppliance: "MyApplianceName",
				DpDomain: "MyDomain", DpFilestore: "local:"}
			assert.DeepEqual(t, "GetList", itemList[0], model.Item{Name: ".."})
			assert.DeepEqual(t, "GetList", itemList[1],
				model.Item{Name: "example-Firewall", Modified: "modified",
					Config: &model.ItemConfig{Type: model.ItemDpObject,
						Name: "example-Firewall", Path: "XMLFirewallService",
						DpAppliance: "MyApplianceName", DpDomain: "MyDomain",
						DpObjectState: model.ItemDpObjectState{OpState: "up",
							AdminState: "enabled", EventCode: "0x00000000",
							ConfigState: "modified"},
						Parent: &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[3],
				model.Item{Name: "example-Firewall3", Size: "down", Modified: "new",
					Config: &model.ItemConfig{Type: model.ItemDpObject,
						Name: "example-Firewall3", Path: "XMLFirewallService",
						DpAppliance: "MyApplianceName", DpDomain: "MyDomain",
						DpObjectState: model.ItemDpObjectState{OpState: "down",
							AdminState: "enabled", EventCode: "0x00b30002",
							ErrorCode:   "Failed to install on port",
							ConfigState: "new"},
						Parent: &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[6],
				model.Item{Name: "get_internal_js_xmlfw", Modified: "",
					Config: &model.ItemConfig{Type: model.ItemDpObject,
						Name: "get_internal_js_xmlfw", Path: "XMLFirewallService",
						DpAppliance: "MyApplianceName", DpDomain: "MyDomain",
						DpObjectState: model.ItemDpObjectState{OpState: "up",
							AdminState: "enabled", EventCode: "0x00000000",
							ConfigState: "saved"},
						Parent: &parentItemConfig}})
		}
	})

	t.Run("ObjectConfigMode/ObjectList SOMA", func(t *testing.T) {
		clearRepo()

		dpa := config.DataPowerAppliance{
			SomaUrl:  testSomaURL,
			Username: "user",
			Domain:   "xxx",
		}
		Repo.ObjectConfigMode = true
		Repo.req = mockRequester{}

		itemToShow := model.ItemConfig{Type: model.ItemDpObjectClass,
			DpAppliance: "MyApplianceName",
			DpDomain:    "MyDomain", DpFilestore: "local:", Path: "XMLFirewallService"}
		config.Conf.DataPowerAppliances[itemToShow.DpAppliance] = dpa
		itemList, err := Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err, nil)
		assert.Equals(t, "GetList", len(itemList), 9)

		if len(itemList) == 9 {
			parentItemConfig := model.ItemConfig{Type: model.ItemDpObjectClass,
				Path: "XMLFirewallService", DpAppliance: "MyApplianceName",
				DpDomain: "MyDomain", DpFilestore: "local:"}
			assert.DeepEqual(t, "GetList", itemList[0], model.Item{Name: ".."})
			assert.DeepEqual(t, "GetList", itemList[1],
				model.Item{Name: "example-Firewall", Modified: "modified",
					Config: &model.ItemConfig{Type: model.ItemDpObject,
						Name: "example-Firewall", Path: "XMLFirewallService",
						DpAppliance: "MyApplianceName", DpDomain: "MyDomain",
						DpObjectState: model.ItemDpObjectState{OpState: "up",
							AdminState: "enabled", EventCode: "0x00000000",
							ConfigState: "modified"},
						Parent: &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[3],
				model.Item{Name: "example-Firewall3", Size: "down", Modified: "new",
					Config: &model.ItemConfig{Type: model.ItemDpObject,
						Name: "example-Firewall3", Path: "XMLFirewallService",
						DpAppliance: "MyApplianceName", DpDomain: "MyDomain",
						DpObjectState: model.ItemDpObjectState{OpState: "down",
							AdminState: "enabled", EventCode: "0x00b30002",
							ErrorCode:   "Failed to install on port",
							ConfigState: "new"},
						Parent: &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[6],
				model.Item{Name: "get_internal_js_xmlfw", Modified: "",
					Config: &model.ItemConfig{Type: model.ItemDpObject,
						Name: "get_internal_js_xmlfw", Path: "XMLFirewallService",
						DpAppliance: "MyApplianceName", DpDomain: "MyDomain",
						DpObjectState: model.ItemDpObjectState{OpState: "up",
							AdminState: "enabled", EventCode: "0x00000000",
							ConfigState: "saved"},
						Parent: &parentItemConfig}})
		}
	})

	t.Run("FilestoreMode/ApplianceList", func(t *testing.T) {
		clearRepo()
		Repo.req = mockRequester{}

		for aplianceName := range config.Conf.DataPowerAppliances {
			delete(config.Conf.DataPowerAppliances, aplianceName)
		}

		itemToShow := model.ItemConfig{Type: model.ItemNone}
		itemList, err := Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err,
			errs.Error("No appliances found, have to configure dpcmder with command line params first."))
		assert.Equals(t, "GetList", len(itemList), 0)

		dpa1 := config.DataPowerAppliance{
			RestUrl:  testRestURL,
			Username: "user",
			Domain:   "Dpa1Domain",
		}
		dpa2 := config.DataPowerAppliance{
			SomaUrl:  testSomaURL,
			Username: "user",
		}
		config.Conf.DataPowerAppliances["dpa1"] = dpa1
		config.Conf.DataPowerAppliances["dpa2"] = dpa2

		itemList, err = Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err, nil)
		assert.Equals(t, "GetList", len(itemList), 2)

		if len(itemList) == 2 {
			parentItemConfig := model.ItemConfig{Type: model.ItemNone}
			assert.DeepEqual(t, "GetList", itemList[0],
				model.Item{Name: "dpa1",
					Config: &model.ItemConfig{Type: model.ItemDpConfiguration,
						Name:        "dpa1",
						DpAppliance: "dpa1", DpDomain: "Dpa1Domain",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[1],
				model.Item{Name: "dpa2",
					Config: &model.ItemConfig{Type: model.ItemDpConfiguration,
						Name:          "dpa2",
						DpAppliance:   "dpa2",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
		}
	})

	t.Run("FilestoreMode/DomainList REST", func(t *testing.T) {
		clearRepo()
		Repo.req = mockRequester{}

		itemToShow := model.ItemConfig{Type: model.ItemDpConfiguration}
		itemList, err := Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err,
			errs.Error("DataPower management interface not set."))
		assert.Equals(t, "GetList", len(itemList), 0)

		dpa := config.DataPowerAppliance{
			RestUrl:  testRestURL,
			Username: "user",
			Domain:   "xxx",
		}
		itemToShow.DpAppliance = "MyApplianceName"
		config.Conf.DataPowerAppliances[itemToShow.DpAppliance] = dpa

		itemList, err = Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err, nil)
		assert.Equals(t, "GetList", len(itemList), 3)

		if len(itemList) == 3 {
			parentItemConfig := model.ItemConfig{Type: model.ItemDpConfiguration,
				DpAppliance: "MyApplianceName"}
			assert.DeepEqual(t, "GetList", itemList[0],
				model.Item{Name: "..", Config: nil})

			assert.DeepEqual(t, "GetList", itemList[1],
				model.Item{Name: "default",
					Config: &model.ItemConfig{Type: model.ItemDpDomain,
						Name:        "default",
						DpAppliance: "MyApplianceName", DpDomain: "default",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[2],
				model.Item{Name: "test", Modified: "*",
					Config: &model.ItemConfig{Type: model.ItemDpDomain,
						Name:        "test",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
		}
	})

	t.Run("FilestoreMode/DomainList SOAP", func(t *testing.T) {
		clearRepo()
		Repo.req = mockRequester{}

		itemToShow := model.ItemConfig{Type: model.ItemDpConfiguration}
		itemList, err := Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err,
			errs.Error("DataPower management interface not set."))
		assert.Equals(t, "GetList", len(itemList), 0)

		dpa := config.DataPowerAppliance{
			SomaUrl:  testSomaURL,
			Username: "user",
			Domain:   "xxx",
		}
		itemToShow.DpAppliance = "MyApplianceName"
		config.Conf.DataPowerAppliances[itemToShow.DpAppliance] = dpa

		itemList, err = Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err, nil)
		assert.Equals(t, "GetList", len(itemList), 3)

		if len(itemList) == 3 {
			parentItemConfig := model.ItemConfig{Type: model.ItemDpConfiguration,
				DpAppliance: "MyApplianceName"}
			assert.DeepEqual(t, "GetList", itemList[0],
				model.Item{Name: "..", Config: nil})

			assert.DeepEqual(t, "GetList", itemList[1],
				model.Item{Name: "default",
					Config: &model.ItemConfig{Type: model.ItemDpDomain,
						Name:        "default",
						DpAppliance: "MyApplianceName", DpDomain: "default",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[2],
				model.Item{Name: "test", Modified: "*",
					Config: &model.ItemConfig{Type: model.ItemDpDomain,
						Name:        "test",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
		}
	})

	t.Run("FilestoreMode/FilestoreList (for appliance) REST", func(t *testing.T) {
		clearRepo()
		Repo.req = mockRequester{}

		itemToShow := model.ItemConfig{Type: model.ItemDpConfiguration,
			DpDomain: "test"}
		itemList, err := Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err,
			errs.Error("DataPower management interface not set."))
		assert.Equals(t, "GetList", len(itemList), 0)

		dpa := config.DataPowerAppliance{
			RestUrl:  testRestURL,
			Username: "user",
			Domain:   "xxx",
		}
		itemToShow.DpAppliance = "MyApplianceName"
		config.Conf.DataPowerAppliances[itemToShow.DpAppliance] = dpa

		itemList, err = Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err, nil)
		assert.Equals(t, "GetList", len(itemList), 13)

		if len(itemList) == 13 {
			parentItemConfig := model.ItemConfig{Type: model.ItemDpConfiguration,
				DpAppliance: "MyApplianceName", DpDomain: "test"}
			assert.DeepEqual(t, "GetList", itemList[0],
				model.Item{Name: "..", Config: nil})

			assert.DeepEqual(t, "GetList", itemList[1],
				model.Item{Name: "cert:",
					Config: &model.ItemConfig{Type: model.ItemDpFilestore,
						Name: "cert:", Path: "cert:",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "cert:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[5],
				model.Item{Name: "local:",
					Config: &model.ItemConfig{Type: model.ItemDpFilestore,
						Name: "local:", Path: "local:",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "local:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[12],
				model.Item{Name: "temporary:",
					Config: &model.ItemConfig{Type: model.ItemDpFilestore,
						Name: "temporary:", Path: "temporary:",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "temporary:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
		}
	})

	t.Run("FilestoreMode/FilestoreList (for appliance) SOMA", func(t *testing.T) {
		clearRepo()
		Repo.req = mockRequester{}

		itemToShow := model.ItemConfig{Type: model.ItemDpConfiguration,
			DpDomain: "test"}
		itemList, err := Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err,
			errs.Error("DataPower management interface not set."))
		assert.Equals(t, "GetList", len(itemList), 0)

		dpa := config.DataPowerAppliance{
			SomaUrl:  testSomaURL,
			Username: "user",
			Domain:   "xxx",
		}
		itemToShow.DpAppliance = "MyApplianceName"
		config.Conf.DataPowerAppliances[itemToShow.DpAppliance] = dpa

		itemList, err = Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err, nil)
		assert.Equals(t, "GetList", len(itemList), 13)

		if len(itemList) == 13 {
			parentItemConfig := model.ItemConfig{Type: model.ItemDpConfiguration,
				DpAppliance: "MyApplianceName", DpDomain: "test"}
			assert.DeepEqual(t, "GetList", itemList[0],
				model.Item{Name: "..", Config: nil})

			assert.DeepEqual(t, "GetList", itemList[1],
				model.Item{Name: "cert:",
					Config: &model.ItemConfig{Type: model.ItemDpFilestore,
						Name: "cert:", Path: "cert:",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "cert:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[5],
				model.Item{Name: "local:",
					Config: &model.ItemConfig{Type: model.ItemDpFilestore,
						Name: "local:", Path: "local:",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "local:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[12],
				model.Item{Name: "temporary:",
					Config: &model.ItemConfig{Type: model.ItemDpFilestore,
						Name: "temporary:", Path: "temporary:",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "temporary:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
		}
	})

	t.Run("FilestoreMode/FilestoreList (for domain) REST", func(t *testing.T) {
		clearRepo()
		Repo.req = mockRequester{}

		itemToShow := model.ItemConfig{Type: model.ItemDpDomain,
			DpDomain: "test"}
		itemList, err := Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err,
			errs.Error("DataPower management interface not set."))
		assert.Equals(t, "GetList", len(itemList), 0)

		dpa := config.DataPowerAppliance{
			RestUrl:  testRestURL,
			Username: "user",
			Domain:   "xxx",
		}
		itemToShow.DpAppliance = "MyApplianceName"
		config.Conf.DataPowerAppliances[itemToShow.DpAppliance] = dpa

		itemList, err = Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err, nil)
		assert.Equals(t, "GetList", len(itemList), 13)

		if len(itemList) == 13 {
			parentItemConfig := model.ItemConfig{Type: model.ItemDpDomain,
				DpAppliance: "MyApplianceName", DpDomain: "test"}
			assert.DeepEqual(t, "GetList", itemList[0],
				model.Item{Name: "..", Config: nil})

			assert.DeepEqual(t, "GetList", itemList[1],
				model.Item{Name: "cert:",
					Config: &model.ItemConfig{Type: model.ItemDpFilestore,
						Name: "cert:", Path: "cert:",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "cert:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[5],
				model.Item{Name: "local:",
					Config: &model.ItemConfig{Type: model.ItemDpFilestore,
						Name: "local:", Path: "local:",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "local:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[12],
				model.Item{Name: "temporary:",
					Config: &model.ItemConfig{Type: model.ItemDpFilestore,
						Name: "temporary:", Path: "temporary:",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "temporary:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
		}
	})

	t.Run("FilestoreMode/FilestoreList (for domain) SOMA", func(t *testing.T) {
		clearRepo()
		Repo.req = mockRequester{}

		itemToShow := model.ItemConfig{Type: model.ItemDpDomain,
			DpDomain: "test"}
		itemList, err := Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err,
			errs.Error("DataPower management interface not set."))
		assert.Equals(t, "GetList", len(itemList), 0)

		dpa := config.DataPowerAppliance{
			SomaUrl:  testSomaURL,
			Username: "user",
			Domain:   "xxx",
		}
		itemToShow.DpAppliance = "MyApplianceName"
		config.Conf.DataPowerAppliances[itemToShow.DpAppliance] = dpa

		itemList, err = Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err, nil)
		assert.Equals(t, "GetList", len(itemList), 13)

		if len(itemList) == 13 {
			parentItemConfig := model.ItemConfig{Type: model.ItemDpDomain,
				DpAppliance: "MyApplianceName", DpDomain: "test"}
			assert.DeepEqual(t, "GetList", itemList[0],
				model.Item{Name: "..", Config: nil})

			assert.DeepEqual(t, "GetList", itemList[1],
				model.Item{Name: "cert:",
					Config: &model.ItemConfig{Type: model.ItemDpFilestore,
						Name: "cert:", Path: "cert:",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "cert:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[5],
				model.Item{Name: "local:",
					Config: &model.ItemConfig{Type: model.ItemDpFilestore,
						Name: "local:", Path: "local:",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "local:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[12],
				model.Item{Name: "temporary:",
					Config: &model.ItemConfig{Type: model.ItemDpFilestore,
						Name: "temporary:", Path: "temporary:",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "temporary:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
		}
	})

	t.Run("FilestoreMode/DirList (for filestore) REST", func(t *testing.T) {
		clearRepo()
		Repo.req = mockRequester{}

		itemToShow := model.ItemConfig{Type: model.ItemDpFilestore,
			DpDomain: "test", DpFilestore: "store:", Path: "store:"}
		itemList, err := Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err,
			errs.Error("DataPower management interface not set."))
		assert.Equals(t, "GetList", len(itemList), 0)

		dpa := config.DataPowerAppliance{
			RestUrl:  testRestURL,
			Username: "user",
			Domain:   "xxx",
		}
		itemToShow.DpAppliance = "MyApplianceName"
		config.Conf.DataPowerAppliances[itemToShow.DpAppliance] = dpa

		itemList, err = Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err, nil)
		assert.Equals(t, "GetList", len(itemList), 189)

		if len(itemList) == 189 {
			parentItemConfig := model.ItemConfig{Type: model.ItemDpFilestore,
				DpAppliance: "MyApplianceName",
				DpDomain:    "test",
				DpFilestore: "store:",
				Path:        "store:"}
			assert.DeepEqual(t, "GetList", itemList[0],
				model.Item{Name: "..", Config: nil})

			assert.DeepEqual(t, "GetList", itemList[1],
				model.Item{Name: "gatewayscript",
					Config: &model.ItemConfig{Type: model.ItemDirectory,
						Name: "gatewayscript", Path: "store:/gatewayscript",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "store:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[12],
				model.Item{Name: "AAAInfo.xsd",
					Size: "18692", Modified: "2019-08-09 15:24:41",
					Config: &model.ItemConfig{Type: model.ItemFile,
						Name: "AAAInfo.xsd", Path: "store:/AAAInfo.xsd",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "store:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[188],
				model.Item{Name: "XSS-Patterns.xml",
					Size: "1086", Modified: "2019-08-09 15:24:41",
					Config: &model.ItemConfig{Type: model.ItemFile,
						Name: "XSS-Patterns.xml", Path: "store:/XSS-Patterns.xml",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "store:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
		}
	})

	t.Run("FilestoreMode/DirList (for filestore) SOMA", func(t *testing.T) {
		clearRepo()
		Repo.req = mockRequester{}

		itemToShow := model.ItemConfig{Type: model.ItemDpFilestore,
			DpDomain: "test", DpFilestore: "store:", Path: "store:"}
		itemList, err := Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err,
			errs.Error("DataPower management interface not set."))
		assert.Equals(t, "GetList", len(itemList), 0)

		dpa := config.DataPowerAppliance{
			SomaUrl:  testSomaURL,
			Username: "user",
			Domain:   "xxx",
		}
		itemToShow.DpAppliance = "MyApplianceName"
		config.Conf.DataPowerAppliances[itemToShow.DpAppliance] = dpa

		itemList, err = Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err, nil)
		assert.Equals(t, "GetList", len(itemList), 191)

		if len(itemList) == 191 {
			parentItemConfig := model.ItemConfig{Type: model.ItemDpFilestore,
				DpAppliance: "MyApplianceName",
				DpDomain:    "test",
				DpFilestore: "store:",
				Path:        "store:"}
			assert.DeepEqual(t, "GetList", itemList[0],
				model.Item{Name: "..", Config: nil})

			assert.DeepEqual(t, "GetList", itemList[2],
				model.Item{Name: "gatewayscript",
					Config: &model.ItemConfig{Type: model.ItemDirectory,
						Name: "gatewayscript", Path: "store:/gatewayscript",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "store:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[14],
				model.Item{Name: "AAAInfo.xsd",
					Size: "18692", Modified: "2019-08-09 15:24:41",
					Config: &model.ItemConfig{Type: model.ItemFile,
						Name: "AAAInfo.xsd", Path: "store:/AAAInfo.xsd",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "store:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[190],
				model.Item{Name: "XSS-Patterns.xml",
					Size: "1086", Modified: "2019-08-09 15:24:41",
					Config: &model.ItemConfig{Type: model.ItemFile,
						Name: "XSS-Patterns.xml", Path: "store:/XSS-Patterns.xml",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "store:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
		}
	})

	t.Run("FilestoreMode/DirList (for dir) REST", func(t *testing.T) {
		clearRepo()
		Repo.req = mockRequester{}

		itemToShow := model.ItemConfig{Type: model.ItemDirectory,
			DpDomain: "test", DpFilestore: "store:", Path: "store:/gatewayscript"}
		itemList, err := Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err,
			errs.Error("DataPower management interface not set."))
		assert.Equals(t, "GetList", len(itemList), 0)

		dpa := config.DataPowerAppliance{
			RestUrl:  testRestURL,
			Username: "user",
			Domain:   "xxx",
		}
		itemToShow.DpAppliance = "MyApplianceName"
		config.Conf.DataPowerAppliances[itemToShow.DpAppliance] = dpa

		itemList, err = Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err, nil)
		assert.Equals(t, "GetList", len(itemList), 30)

		if len(itemList) == 30 {
			parentItemConfig := model.ItemConfig{Type: model.ItemDirectory,
				DpAppliance: "MyApplianceName",
				DpDomain:    "test",
				DpFilestore: "store:",
				Path:        "store:/gatewayscript"}
			assert.DeepEqual(t, "GetList", itemList[0],
				model.Item{Name: "..", Config: nil})

			assert.DeepEqual(t, "GetList", itemList[1],
				model.Item{Name: "example-b2b-routing.js",
					Size: "2413", Modified: "2019-08-09 15:24:42",
					Config: &model.ItemConfig{Type: model.ItemFile,
						Name:        "example-b2b-routing.js",
						Path:        "store:/gatewayscript/example-b2b-routing.js",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "store:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[29],
				model.Item{Name: "example-xml-xslt.js",
					Size: "1740", Modified: "2019-08-09 15:24:42",
					Config: &model.ItemConfig{Type: model.ItemFile,
						Name: "example-xml-xslt.js", Path: "store:/gatewayscript/example-xml-xslt.js",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "store:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
		}
	})

	t.Run("FilestoreMode/DirList (for dir) SOMA", func(t *testing.T) {
		clearRepo()
		Repo.req = mockRequester{}

		itemToShow := model.ItemConfig{Type: model.ItemDirectory,
			DpDomain: "test", DpFilestore: "store:", Path: "store:/gatewayscript"}
		itemList, err := Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err,
			errs.Error("DataPower management interface not set."))
		assert.Equals(t, "GetList", len(itemList), 0)

		dpa := config.DataPowerAppliance{
			SomaUrl:  testSomaURL,
			Username: "user",
			Domain:   "xxx",
		}
		itemToShow.DpAppliance = "MyApplianceName"
		config.Conf.DataPowerAppliances[itemToShow.DpAppliance] = dpa

		itemList, err = Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err, nil)
		assert.Equals(t, "GetList", len(itemList), 30)

		if len(itemList) == 30 {
			parentItemConfig := model.ItemConfig{Type: model.ItemDirectory,
				DpAppliance: "MyApplianceName",
				DpDomain:    "test",
				DpFilestore: "store:",
				Path:        "store:/gatewayscript"}
			assert.DeepEqual(t, "GetList", itemList[0],
				model.Item{Name: "..", Config: nil})

			assert.DeepEqual(t, "GetList", itemList[1],
				model.Item{Name: "example-b2b-routing.js",
					Size: "2413", Modified: "2019-08-09 15:24:42",
					Config: &model.ItemConfig{Type: model.ItemFile,
						Name:        "example-b2b-routing.js",
						Path:        "store:/gatewayscript/example-b2b-routing.js",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "store:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
			assert.DeepEqual(t, "GetList", itemList[29],
				model.Item{Name: "example-xml-xslt.js",
					Size: "1740", Modified: "2019-08-09 15:24:42",
					Config: &model.ItemConfig{Type: model.ItemFile,
						Name: "example-xml-xslt.js", Path: "store:/gatewayscript/example-xml-xslt.js",
						DpAppliance: "MyApplianceName", DpDomain: "test",
						DpFilestore:   "store:",
						DpObjectState: model.ItemDpObjectState{},
						Parent:        &parentItemConfig}})
		}
	})

	t.Run("FilestoreMode/List Object type", func(t *testing.T) {
		clearRepo()
		Repo.req = mockRequester{}

		itemToShow := model.ItemConfig{Type: model.ItemDpObjectClassList,
			DpDomain: "test", DpFilestore: "store:", Path: "store:/gatewayscript"}
		_, err := Repo.GetList(&itemToShow)

		assert.Equals(t, "GetList", err,
			errs.Error("Internal error showing filestore mode - wrong view type."))
	})

}

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
			Name: "dir2", Path: "/dir1/dir2",
			Parent: &model.ItemConfig{Type: model.ItemDpDomain, DpDomain: "default"}}, "",
			&model.ItemConfig{Type: model.ItemDpDomain, DpDomain: "default"}, nil},
		{&model.ItemConfig{
			Type: model.ItemDpFilestore, Name: "cert:", DpDomain: "default", DpFilestore: "cert:",
			Parent: &model.ItemConfig{Type: model.ItemDpDomain, Name: "default", DpDomain: "default"}},
			"local:/dirA/dirB",
			&model.ItemConfig{
				Type: model.ItemDirectory, DpDomain: "default", DpFilestore: "local:",
				Name: "dirB", Path: "local:/dirA/dirB",
				Parent: &model.ItemConfig{
					Type: model.ItemDirectory, DpDomain: "default", DpFilestore: "local:",
					Name: "dirA", Path: "local:/dirA",
					Parent: &model.ItemConfig{
						Type: model.ItemDpFilestore, DpDomain: "default", DpFilestore: "local:",
						Name: "local:", Path: "local:", Parent: &model.ItemConfig{
							Type: model.ItemDpDomain, Name: "default", DpDomain: "default"}}}},
			nil},
	}

	for idx, testCase := range testDataMatrix {
		newView, err := Repo.GetViewConfigByPath(testCase.currentView, testCase.dirPath)
		if !reflect.DeepEqual(newView, testCase.newView) {
			t.Errorf("[%d] GetViewConfigByPath(%v, '%s') res: got %v, want %v",
				idx, testCase.currentView, testCase.dirPath, newView, testCase.newView)
		}
		if !reflect.DeepEqual(err, testCase.err) {
			t.Errorf("[%d] GetViewConfigByPath(%v, '%s') err: got %v, want %v",
				idx, testCase.currentView, testCase.dirPath, err, testCase.err)
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
