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
	name            string
	dpFilestoreXml  string
	invalidateCache bool
}

var Repo = DpRepo{name: "DpRepo"}

func InitNetworkSettings() {
	dpnet.InitNetworkSettings()
}

func (r *DpRepo) String() string {
	return r.name
}

func (r *DpRepo) GetInitialItem() model.Item {
	logging.LogDebug("repo/dp/GetInitialItem()")
	initialItem := model.Item{
		Config: &model.ItemConfig{Type: model.ItemNone}}
	return initialItem
}

func (r *DpRepo) GetTitle(itemToShow model.Item) string {
	logging.LogDebugf("repo/dp/GetTitle(%v)", itemToShow)
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
	logging.LogDebugf("repo/dp/GetList(%v)", itemToShow)

	switch itemToShow.Config.Type {
	case model.ItemNone:
		config.ClearDpConfig()
		return listAppliances()
	case model.ItemDpConfiguration:
		config.LoadDpConfig(itemToShow.Config.DpAppliance)
		if itemToShow.Config.DpDomain != "" {
			return r.listFilestores(itemToShow.Config)
		} else {
			return listDomains(itemToShow.Config)
		}
	case model.ItemDpDomain:
		return r.listFilestores(itemToShow.Config)
	case model.ItemDpFilestore:
		return r.listDpDir(itemToShow.Config)
	case model.ItemDirectory:
		return r.listDpDir(itemToShow.Config)
	default:
		return model.ItemList{}
	}
}

func (r *DpRepo) InvalidateCache() {
	if config.DpUseSoma() {
		r.invalidateCache = true
	}
}

// listAppliances returns ItemList of DataPower appliance Items from configuration.
func listAppliances() model.ItemList {
	appliances := config.Conf.DataPowerAppliances
	logging.LogDebugf("repo/dp/listAppliances(), appliances: %v", appliances)

	appliancesConfig := model.ItemConfig{Type: model.ItemNone}
	items := make(model.ItemList, len(appliances))
	idx := 0
	for name, config := range appliances {
		itemConfig := model.ItemConfig{Type: model.ItemDpConfiguration, DpAppliance: name, DpDomain: config.Domain, Parent: &appliancesConfig}
		items[idx] = model.Item{Name: name, Config: &itemConfig}
		idx = idx + 1
	}

	sort.Sort(items)
	logging.LogDebugf("repo/dp/listAppliances(), items: %v", items)

	return items
}

// listDomains loads DataPower domains from current DataPower.
func listDomains(selectedItemConfig *model.ItemConfig) model.ItemList {
	logging.LogDebugf("repo/dp/listDomains('%s')", selectedItemConfig)
	domainNames := fetchDpDomains()
	logging.LogDebugf("repo/dp/listDomains('%s'), domainNames: %v", selectedItemConfig, domainNames)

	items := make(model.ItemList, len(domainNames)+1)
	items[0] = model.Item{Name: "..", Config: selectedItemConfig.Parent}

	for idx, name := range domainNames {
		itemConfig := model.ItemConfig{Type: model.ItemDpDomain,
			DpAppliance: selectedItemConfig.DpAppliance, DpDomain: name, Parent: selectedItemConfig}
		items[idx+1] = model.Item{Name: name, Config: &itemConfig}
	}

	sort.Sort(items)

	return items
}

// listFilestores loads DataPower filestores in current domain (cert:, local:,..).
func (r *DpRepo) listFilestores(selectedItemConfig *model.ItemConfig) model.ItemList {
	logging.LogDebugf("repo/dp/listFilestores('%s')", selectedItemConfig)
	if config.DpUseRest() {
		jsonString := dpnet.RestGet("/mgmt/filestore/" + selectedItemConfig.DpDomain)
		// println("jsonString: " + jsonString)

		// .filestore.location[]?.name
		doc, err := jsonquery.Parse(strings.NewReader(jsonString))
		if err != nil {
			logging.LogFatal(err)
		}
		filestoreNameNodes := jsonquery.Find(doc, "/filestore/location/*/name")

		items := make(model.ItemList, len(filestoreNameNodes)+1)
		items[0] = model.Item{Name: "..", Config: selectedItemConfig.Parent}

		for idx, node := range filestoreNameNodes {
			// "local:"
			filestoreName := node.InnerText()
			itemConfig := model.ItemConfig{Type: model.ItemDpFilestore, DpAppliance: selectedItemConfig.DpAppliance,
				DpDomain: selectedItemConfig.DpDomain, Path: filestoreName, Parent: selectedItemConfig}
			items[idx+1] = model.Item{Name: filestoreName, Config: &itemConfig}
		}

		sort.Sort(items)

		return items
	} else if config.DpUseSoma() {
		somaRequest := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"><soapenv:Body>" +
			"<dp:request xmlns:dp=\"http://www.datapower.com/schemas/management\" domain=\"" + selectedItemConfig.DpDomain + "\">" +
			"<dp:get-filestore layout-only=\"true\" no-subdirectories=\"true\"/></dp:request>" +
			"</soapenv:Body></soapenv:Envelope>"
		// In SOMA response we receive whole hierarchy of subdirectories and subfiles.
		// TODO - check if it would be better to fetch each filestore hierarchy when needed.
		// <xsd:element name="get-filestore">
		// 	<xsd:complexType>
		// 		<xsd:attribute name="location" type="tns:filestore-location"/> - enum (local:, store:,..)
		// 		<xsd:attribute name="annotated" type="xsd:boolean"/>
		// 		<xsd:attribute name="layout-only" type="xsd:boolean"/>
		// 		<xsd:attribute name="no-subdirectories" type="xsd:boolean"/>
		// 	</xsd:complexType>
		// </xsd:element>
		dpFilestoresXML := dpnet.Soma(somaRequest)
		doc, err := xmlquery.Parse(strings.NewReader(dpFilestoresXML))
		if err != nil {
			logging.LogFatal(err)
		}
		filestoreNameNodes := xmlquery.Find(doc, "//*[local-name()='location']/@name")

		items := make(model.ItemList, len(filestoreNameNodes)+1)
		items[0] = model.Item{Name: "..", Config: selectedItemConfig.Parent}

		for idx, node := range filestoreNameNodes {
			// "local:"
			filestoreName := node.InnerText()
			itemConfig := model.ItemConfig{Type: model.ItemDpFilestore, DpAppliance: selectedItemConfig.DpAppliance,
				DpDomain: selectedItemConfig.DpDomain, Path: filestoreName, Parent: selectedItemConfig}
			items[idx+1] = model.Item{Name: filestoreName, Config: &itemConfig}
		}

		sort.Sort(items)

		return items
	}

	logging.LogFatal("repo/dp/listFilestores(), unknown Dp management interface.")
	return nil
}

// listDpDir loads DataPower directory (local:, local:///test,..).
func (r *DpRepo) listDpDir(selectedItemConfig *model.ItemConfig) model.ItemList {
	logging.LogDebugf("repo/dp/listDpDir('%s')", selectedItemConfig)
	parentDir := model.Item{Name: "..", Config: selectedItemConfig.Parent}
	filesDirs := r.listFiles(selectedItemConfig)

	itemsWithParentDir := make([]model.Item, 0)
	itemsWithParentDir = append(itemsWithParentDir, parentDir)
	itemsWithParentDir = append(itemsWithParentDir, filesDirs...)

	return itemsWithParentDir
}

func (r *DpRepo) listFiles(selectedItemConfig *model.ItemConfig) []model.Item {
	logging.LogDebugf("repo/dp/listFiles('%s')", selectedItemConfig)

	if config.DpUseRest() {
		items := make(model.ItemList, 0)
		currRestDirPath := strings.Replace(selectedItemConfig.Path, ":", "", 1)
		jsonString := dpnet.RestGet("/mgmt/filestore/" + selectedItemConfig.DpDomain + "/" + currRestDirPath)
		// println("jsonString: " + jsonString)

		doc, err := jsonquery.Parse(strings.NewReader(jsonString))
		if err != nil {
			logging.LogFatal(err)
		}

		// "//" - work-around - for one directory we get JSON object, for multiple directories we get JSON array
		dirNodes := jsonquery.Find(doc, "/filestore/location/directory//name/..")
		for _, n := range dirNodes {
			dirDpPath := n.SelectElement("name").InnerText()
			_, dirName := utils.SplitOnLast(dirDpPath, "/")
			itemConfig := model.ItemConfig{Type: model.ItemDirectory,
				DpAppliance: selectedItemConfig.DpAppliance, DpDomain: selectedItemConfig.DpDomain,
				Path: dirDpPath, Parent: selectedItemConfig}
			item := model.Item{Name: dirName, Config: &itemConfig}
			items = append(items, item)
		}

		// "//" - work-around - for one file we get JSON object, for multiple files we get JSON array
		fileNodes := jsonquery.Find(doc, "/filestore/location/file//name/..")
		for _, n := range fileNodes {
			fileName := n.SelectElement("name").InnerText()
			fileSize := n.SelectElement("size").InnerText()
			fileModified := n.SelectElement("modified").InnerText()
			itemConfig := model.ItemConfig{Type: model.ItemFile,
				DpAppliance: selectedItemConfig.DpAppliance, DpDomain: selectedItemConfig.DpDomain,
				Path: utils.GetDpPath(selectedItemConfig.Path, fileName), Parent: selectedItemConfig}
			item := model.Item{Name: fileName, Size: fileSize, Modified: fileModified, Config: &itemConfig}
			items = append(items, item)
		}

		sort.Sort(items)
		return items
	} else if config.DpUseSoma() {
		dpFilestoreLocation, _ := utils.SplitOnFirst(selectedItemConfig.Path, "/")
		dpFilestoreIsRoot := !strings.Contains(selectedItemConfig.Path, "/")
		var dpDirNodes []*xmlquery.Node
		var dpFileNodes []*xmlquery.Node

		// If we open filestore or open file but want to reload - refresh current filestore XML cache.
		if dpFilestoreIsRoot || r.invalidateCache {
			somaRequest := "<soapenv:Envelope xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"><soapenv:Body>" +
				"<dp:request xmlns:dp=\"http://www.datapower.com/schemas/management\" domain=\"" + selectedItemConfig.DpDomain + "\">" +
				"<dp:get-filestore layout-only=\"false\" no-subdirectories=\"false\" location=\"" + dpFilestoreLocation + "\"/></dp:request>" +
				"</soapenv:Body></soapenv:Envelope>"
			r.dpFilestoreXml = dpnet.Soma(somaRequest)
			r.invalidateCache = false
		}

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
			dpDirNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']//directory[@name='"+selectedItemConfig.Path+"']/directory")
			dpFileNodes = xmlquery.Find(doc, "//*[local-name()='location' and @name='"+dpFilestoreLocation+"']//directory[@name='"+selectedItemConfig.Path+"']/file")
		}

		dirNum := len(dpDirNodes)
		fileNum := len(dpFileNodes)
		items := make(model.ItemList, dirNum+fileNum)
		for idx, node := range dpDirNodes {
			// "local:"
			dirFullName := node.SelectAttr("name")
			_, dirName := utils.SplitOnLast(dirFullName, "/")
			itemConfig := model.ItemConfig{Type: model.ItemDirectory,
				DpAppliance: selectedItemConfig.DpAppliance, DpDomain: selectedItemConfig.DpDomain,
				Path: dirFullName, Parent: selectedItemConfig}
			// Path: selectedItemConfig.Path
			items[idx] = model.Item{Name: dirName, Config: &itemConfig}
		}

		for idx, node := range dpFileNodes {
			// "local:"
			fileName := node.SelectAttr("name")
			fileSize := node.SelectElement("size").InnerText()
			fileModified := node.SelectElement("modified").InnerText()
			itemConfig := model.ItemConfig{Type: model.ItemFile,
				DpAppliance: selectedItemConfig.DpAppliance, DpDomain: selectedItemConfig.DpDomain,
				Path: selectedItemConfig.Path, Parent: selectedItemConfig}
			// selectedItemConfig.Path
			items[idx+dirNum] = model.Item{Name: fileName, Size: fileSize, Modified: fileModified, Config: &itemConfig}
		}

		sort.Sort(items)
		return items
	} else {
		logging.LogDebug("repo/dp/listFiles(), using neither REST neither SOMA.")
		return model.ItemList{}
	}
}

func findItemConfigParentDomain(itemConfig *model.ItemConfig) *model.ItemConfig {
	if itemConfig.Type == model.ItemDpDomain {
		return itemConfig
	}
	if itemConfig.Parent == nil {
		return nil
	}
	return findItemConfigParentDomain(itemConfig.Parent)
}

func fetchDpDomains() []string {
	logging.LogDebug("repo/dp/fetchDpDomains()")
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
