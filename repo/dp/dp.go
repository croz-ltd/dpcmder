package dp

import (
	"fmt"
	"github.com/antchfx/jsonquery"
	"github.com/antchfx/xmlquery"
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/repo/dp/internal/dpnet"
	"github.com/croz-ltd/dpcmder/utils"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"sort"
	"strings"
)

type DpRepo struct {
	name           string
	dpFilestoreXml string
}

var Repo = DpRepo{name: "DpRepo"}

func InitNetworkSettings() {
	dpnet.InitNetworkSettings()
}

func (r *DpRepo) GetInitialItem() model.Item {
	logging.LogDebug("repo/dp/GetInitialItem()")
	initialItem := model.Item{
		Config: &model.ItemConfig{Type: model.ItemNone}}
	return initialItem
}

func (r *DpRepo) GetTitle(itemToShow model.Item) string {
	logging.LogDebug(fmt.Sprintf("repo/dp/GetTitle(%v)", itemToShow))
	dpDomain := itemToShow.Config.DpDomain
	currPath := itemToShow.Config.Path

	var url *string
	if *config.DpRestURL != "" {
		url = config.DpRestURL
	} else {
		url = config.DpSomaURL
	}

	return fmt.Sprintf("%s @ %s (%s) %s", *config.DpUsername, *url, dpDomain, currPath)
}
func (r *DpRepo) GetList(itemToShow model.Item) model.ItemList {
	logging.LogDebug(fmt.Sprintf("repo/dp/GetList(%v)", itemToShow))

	switch itemToShow.Config.Type {
	case model.ItemNone:
		config.ClearDpConfig()
		return r.listAppliances()
	case model.ItemDpConfiguration:
		config.LoadDpConfig(itemToShow.Config.DpAppliance)
		if itemToShow.Config.DpDomain != "" {
			return r.listFilestores(itemToShow)
		} else {
			return r.listDomains(itemToShow)
		}
	case model.ItemDpDomain:
		return r.listFilestores(itemToShow)
	case model.ItemDpFilestore:
		return r.listDpDir(itemToShow)
	case model.ItemDirectory:
		return r.listDpDir(itemToShow)
	default:
		return model.ItemList{}
	}
}

// listAppliances returns ItemList of DataPower appliance Items from configuration.
func (r *DpRepo) listAppliances() model.ItemList {
	appliances := config.Conf.DataPowerAppliances
	logging.LogDebug(fmt.Sprintf("repo/dp/listAppliances(), appliances: %v", appliances))

	appliancesConfig := model.ItemConfig{Type: model.ItemNone}
	items := make(model.ItemList, len(appliances))
	idx := 0
	for name, config := range appliances {
		itemConfig := model.ItemConfig{Type: model.ItemDpConfiguration, DpAppliance: name, DpDomain: config.Domain, Parent: &appliancesConfig}
		items[idx] = model.Item{Name: name, Config: &itemConfig}
		idx = idx + 1
	}

	sort.Sort(items)
	logging.LogDebug(fmt.Sprintf("repo/dp/listAppliances(), items: %v", items))

	return items
}

// listDomains loads DataPower domains from current DataPower.
func (r *DpRepo) listDomains(selectedItem model.Item) model.ItemList {
	logging.LogDebug(fmt.Sprintf("repo/dp/listDomains('%s')", selectedItem))
	domainNames := fetchDpDomains()
	logging.LogDebug(fmt.Sprintf("repo/dp/listDomains('%s'), domainNames: %v", selectedItem, domainNames))

	items := make(model.ItemList, len(domainNames)+1)
	items[0] = model.Item{Name: "..", Config: selectedItem.Config.Parent}

	for idx, name := range domainNames {
		itemConfig := model.ItemConfig{Type: model.ItemDpDomain,
			DpAppliance: selectedItem.Config.DpAppliance, DpDomain: name, Parent: selectedItem.Config}
		items[idx+1] = model.Item{Name: name, Config: &itemConfig}
	}

	sort.Sort(items)

	return items
}

// listFilestores loads DataPower filestores in current domain (cert:, local:,..).
func (r *DpRepo) listFilestores(selectedItem model.Item) model.ItemList {
	logging.LogDebug(fmt.Sprintf("repo/dp/listFilestores('%s')", selectedItem))
	if config.DpUseRest() {
		jsonString := dpnet.RestGet("/mgmt/filestore/" + selectedItem.Config.DpDomain)
		// println("jsonString: " + jsonString)

		// .filestore.location[]?.name
		doc, err := jsonquery.Parse(strings.NewReader(jsonString))
		if err != nil {
			logging.LogFatal(err)
		}
		filestoreNameNodes := jsonquery.Find(doc, "/filestore/location/*/name")

		items := make(model.ItemList, len(filestoreNameNodes)+1)
		items[0] = model.Item{Name: "..", Config: selectedItem.Config.Parent}

		for idx, node := range filestoreNameNodes {
			// "local:"
			filestoreName := node.InnerText()
			itemConfig := model.ItemConfig{Type: model.ItemDpFilestore, DpAppliance: selectedItem.Config.DpAppliance,
				DpDomain: selectedItem.Config.DpDomain, Path: filestoreName, Parent: selectedItem.Config}
			items[idx+1] = model.Item{Name: filestoreName, Config: &itemConfig}
		}

		sort.Sort(items)

		return items
	} else if config.DpUseSoma() {
		somaRequest := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"><soapenv:Body>" +
			"<dp:request xmlns:dp=\"http://www.datapower.com/schemas/management\" domain=\"" + selectedItem.Config.DpDomain + "\">" +
			"<dp:get-filestore layout-only=\"false\" no-subdirectories=\"false\"/></dp:request>" +
			"</soapenv:Body></soapenv:Envelope>"
		r.dpFilestoreXml = dpnet.Soma(somaRequest)
		doc, err := xmlquery.Parse(strings.NewReader(r.dpFilestoreXml))
		if err != nil {
			logging.LogFatal(err)
		}
		filestoreNameNodes := xmlquery.Find(doc, "//*[local-name()='location']/@name")

		items := make(model.ItemList, len(filestoreNameNodes)+1)
		items[0] = model.Item{Name: "..", Config: selectedItem.Config.Parent}

		for idx, node := range filestoreNameNodes {
			// "local:"
			filestoreName := node.InnerText()
			itemConfig := model.ItemConfig{Type: model.ItemDpFilestore, DpAppliance: selectedItem.Config.DpAppliance,
				DpDomain: selectedItem.Config.DpDomain, Path: filestoreName, Parent: selectedItem.Config}
			items[idx+1] = model.Item{Name: filestoreName, Config: &itemConfig}
		}

		sort.Sort(items)

		return items
	}

	logging.LogFatal("repo/dp/listFilestores(), unknown Dp management interface.")
	return nil
}

// listDpDir loads DataPower directory (local:, local:///test,..).
func (r *DpRepo) listDpDir(selectedItem model.Item) model.ItemList {
	logging.LogDebug(fmt.Sprintf("repo/dp/listDpDir('%s')", selectedItem))
	// var parentType model.ItemType
	// if strings.Contains(currPath, "/") {
	// 	parentType = model.ItemDirectory
	// } else {
	// 	parentType = model.ItemDpDomain
	// }
	parentDir := model.Item{Name: "..", Config: selectedItem.Config.Parent}
	filesDirs := r.listFiles(selectedItem)

	itemsWithParentDir := make([]model.Item, 0)
	itemsWithParentDir = append(itemsWithParentDir, parentDir)
	itemsWithParentDir = append(itemsWithParentDir, filesDirs...)

	return itemsWithParentDir
}

func (r *DpRepo) listFiles(selectedItem model.Item) []model.Item {
	logging.LogDebug(fmt.Sprintf("repo/dp/listFiles('%s')", selectedItem))
	filesDirs := make(model.ItemList, 0)

	if config.DpUseRest() {
		currRestDirPath := strings.Replace(selectedItem.Config.Path, ":", "", 1)
		jsonString := dpnet.RestGet("/mgmt/filestore/" + selectedItem.Config.DpDomain + "/" + currRestDirPath)
		// println("jsonString: " + jsonString)

		doc, err := jsonquery.Parse(strings.NewReader(jsonString))
		if err != nil {
			logging.LogFatal(err)
		}

		// .filestore.location.directory /name
		// work-around - for one directory we get JSON object, for multiple directories we get JSON array
		dirNodes := jsonquery.Find(doc, "/filestore/location/directory//name/..")
		for _, n := range dirNodes {
			dirDpPath := n.SelectElement("name").InnerText()
			_, dirName := utils.SplitOnLast(dirDpPath, "/")
			itemConfig := model.ItemConfig{Type: model.ItemDirectory,
				DpAppliance: selectedItem.Config.DpAppliance, DpDomain: selectedItem.Config.DpDomain,
				Path: dirDpPath, Parent: selectedItem.Config}
			item := model.Item{Name: dirName, Config: &itemConfig}
			filesDirs = append(filesDirs, item)
		}

		// .filestore.location.file      /name, /size, /modified
		fileNodes := jsonquery.Find(doc, "/filestore/location/file//name/..")
		for _, n := range fileNodes {
			fileName := n.SelectElement("name").InnerText()
			fileSize := n.SelectElement("size").InnerText()
			fileModified := n.SelectElement("modified").InnerText()
			itemConfig := model.ItemConfig{Type: model.ItemFile,
				DpAppliance: selectedItem.Config.DpAppliance, DpDomain: selectedItem.Config.DpDomain,
				Path: utils.GetDpPath(selectedItem.Config.Path, fileName), Parent: selectedItem.Config}
			item := model.Item{Name: fileName, Size: fileSize, Modified: fileModified, Config: &itemConfig}
			filesDirs = append(filesDirs, item)
		}
	} else if config.DpUseSoma() {
		dpFilestoreLocation, _ := utils.SplitOnFirst(selectedItem.Config.Path, "/")
		dpFilestoreIsRoot := !strings.Contains(selectedItem.Config.Path, "/")
		var dpDirNodes []*xmlquery.Node
		var dpFileNodes []*xmlquery.Node
		if dpFilestoreIsRoot {
			doc, err := xmlquery.Parse(strings.NewReader(r.dpFilestoreXml))
			if err != nil {
				logging.LogFatal(err)
			}
			dpDirNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']/directory")
			dpFileNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']/file")
			// println(dpFilestoreLocation)
		} else {
			doc, err := xmlquery.Parse(strings.NewReader(r.dpFilestoreXml))
			if err != nil {
				logging.LogFatal(err)
			}
			dpDirNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']//directory[@name='"+selectedItem.Config.Path+"']/directory")
			dpFileNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']//directory[@name='"+selectedItem.Config.Path+"']/file")
		}

		dirNum := len(dpDirNodes)
		items := make(model.ItemList, dirNum+len(dpFileNodes))
		for idx, node := range dpDirNodes {
			// "local:"
			dirFullName := node.SelectAttr("name")
			_, dirName := utils.SplitOnLast(dirFullName, "/")
			itemConfig := model.ItemConfig{Type: model.ItemDirectory,
				DpAppliance: selectedItem.Config.DpAppliance, DpDomain: selectedItem.Config.DpDomain,
				Path: dirFullName, Parent: selectedItem.Config}
			// Path: selectedItem.Config.Path
			items[idx] = model.Item{Name: dirName, Config: &itemConfig}
		}

		for idx, node := range dpFileNodes {
			// "local:"
			fileName := node.SelectAttr("name")
			fileSize := node.SelectElement("size").InnerText()
			fileModified := node.SelectElement("modified").InnerText()
			itemConfig := model.ItemConfig{Type: model.ItemFile,
				DpAppliance: selectedItem.Config.DpAppliance, DpDomain: selectedItem.Config.DpDomain,
				Path: selectedItem.Config.Path, Parent: selectedItem.Config}
			// selectedItem.Config.Path
			items[idx+dirNum] = model.Item{Name: fileName, Size: fileSize, Modified: fileModified, Config: &itemConfig}
		}

		return items
	}

	sort.Sort(filesDirs)

	return filesDirs
}

func fetchDpDomains() []string {
	logging.LogDebug(fmt.Sprintf("repo/dp/fetchDpDomains()"))
	domains := make([]string, 0)

	if config.DpUseRest() {
		bodyString := dpnet.RestGet("/mgmt/domains/config/")

		// .domain[].name
		doc, err := jsonquery.Parse(strings.NewReader(bodyString))
		if err != nil {
			logging.LogFatal(err)
		}
		list := jsonquery.Find(doc, "/domain//name")
		for _, n := range list {
			domains = append(domains, n.InnerText())
		}
	} else if config.DpUseSoma() {
		somaRequest := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\">" +
			"<soapenv:Body><dp:GetDomainListRequest xmlns:dp=\"http://www.datapower.com/schemas/appliance/management/1.0\"/></soapenv:Body>" +
			"</soapenv:Envelope>"
		somaResponse := dpnet.Amp(somaRequest)
		doc, err := xmlquery.Parse(strings.NewReader(somaResponse))
		if err != nil {
			logging.LogFatal(err)
		}
		list := xmlquery.Find(doc, "//*[local-name()='GetDomainListResponse']/*[local-name()='Domain']/text()")
		for _, n := range list {
			domains = append(domains, n.InnerText())
		}
	} else {
		logging.LogDebug("repo/dp/fetchDpDomains(), using neither REST neither SOMA.")
	}

	return domains
}
